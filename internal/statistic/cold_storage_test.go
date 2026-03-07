package statistic

import (
	"fmt"
	"os"
	"path/filepath"
	"ssd/internal/models"
	"ssd/internal/testutil"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestColdStorage(t *testing.T, coldTTL time.Duration) *ColdStorage {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	return NewColdStorage(dir, coldTTL, &testutil.MockCompressor{}, &testutil.MockLogger{})
}

func TestColdStorage_Has_Empty(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	assert.False(t, cs.Has("default", "fp1"))
}

func TestColdStorage_Evict_AddsToIndex(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	data := map[int]*models.StatRecord{1: {Views: 5}}

	cs.Evict("default", "fp1", data)

	assert.True(t, cs.Has("default", "fp1"))
	assert.False(t, cs.Has("default", "fp2"))
	assert.False(t, cs.Has("news", "fp1"))
}

func TestColdStorage_Evict_NoIO(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 1}})

	// No file should exist until Flush
	_, err := os.Stat(cs.coldFilePath("default"))
	assert.True(t, os.IsNotExist(err))
}

func TestColdStorage_RestoreFromPending(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	data := map[int]*models.StatRecord{
		1: {Views: 10, Clicks: 3},
		2: {Views: 1},
	}
	cs.Evict("default", "fp1", data)

	restored, err := cs.Restore("default", "fp1")
	require.NoError(t, err)
	require.NotNil(t, restored)

	assert.Equal(t, 10, restored[1].Views)
	assert.Equal(t, 3, restored[1].Clicks)
	assert.Equal(t, 1, restored[2].Views)

	// Should be removed from index and pending
	assert.False(t, cs.Has("default", "fp1"))
	cs.mu.RLock()
	_, inPending := cs.pending["default"]["fp1"]
	cs.mu.RUnlock()
	assert.False(t, inPending)
}

func TestColdStorage_RestoreNonExistent(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	restored, err := cs.Restore("default", "fp_missing")
	assert.NoError(t, err)
	assert.Nil(t, restored)
}

func TestColdStorage_EvictFlushRestoreRoundtrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}

	cs := NewColdStorage(dir, 0, comp, logger)
	data := map[int]*models.StatRecord{
		1: {Views: 42, Clicks: 7, Ftr: 2},
		5: {Views: 1},
	}
	cs.Evict("default", "fp1", data)
	cs.Evict("news", "fp2", map[int]*models.StatRecord{3: {Views: 100}})

	// Flush to disk
	require.NoError(t, cs.Flush())

	// File should exist
	_, err := os.Stat(cs.coldFilePath("default"))
	assert.NoError(t, err)
	_, err = os.Stat(cs.coldFilePath("news"))
	assert.NoError(t, err)

	// Create a new ColdStorage instance, restore index
	cs2 := NewColdStorage(dir, 0, comp, logger)
	require.NoError(t, cs2.RestoreIndex())

	assert.True(t, cs2.Has("default", "fp1"))
	assert.True(t, cs2.Has("news", "fp2"))

	// Restore from disk
	restored, err := cs2.Restore("default", "fp1")
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, 42, restored[1].Views)
	assert.Equal(t, 7, restored[1].Clicks)
	assert.Equal(t, 2, restored[1].Ftr)
	assert.Equal(t, 1, restored[5].Views)

	// fp1 should no longer be in index
	assert.False(t, cs2.Has("default", "fp1"))
	// fp2 still there
	assert.True(t, cs2.Has("news", "fp2"))
}

func TestColdStorage_LazyDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}

	cs := NewColdStorage(dir, 0, comp, logger)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 1}})
	cs.Evict("default", "fp2", map[int]*models.StatRecord{2: {Views: 2}})
	require.NoError(t, cs.Flush())

	// Restore fp1 (lazy delete)
	_, err := cs.Restore("default", "fp1")
	require.NoError(t, err)

	// fp1 should be in restored set, not yet deleted from file
	cs.mu.RLock()
	_, inRestored := cs.restored["default"]["fp1"]
	cs.mu.RUnlock()
	assert.True(t, inRestored)

	// Flush applies lazy deletes
	require.NoError(t, cs.Flush())

	// Reload and verify fp1 is gone, fp2 remains
	cs2 := NewColdStorage(dir, 0, comp, logger)
	require.NoError(t, cs2.RestoreIndex())

	assert.False(t, cs2.Has("default", "fp1"))
	assert.True(t, cs2.Has("default", "fp2"))
}

func TestColdStorage_ColdTTL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}

	cs := NewColdStorage(dir, 1*time.Hour, comp, logger)
	cs.Evict("default", "fp_old", map[int]*models.StatRecord{1: {Views: 1}})

	// Manually backdate the entry
	cs.mu.Lock()
	cs.pending["default"]["fp_old"].EvictedAt = time.Now().Add(-2 * time.Hour)
	cs.mu.Unlock()

	cs.Evict("default", "fp_new", map[int]*models.StatRecord{2: {Views: 2}})

	// Flush — fp_old should be cleaned by coldTTL
	require.NoError(t, cs.Flush())

	// Reload and verify
	cs2 := NewColdStorage(dir, 1*time.Hour, comp, logger)
	require.NoError(t, cs2.RestoreIndex())

	assert.False(t, cs2.Has("default", "fp_old")) // expired
	assert.True(t, cs2.Has("default", "fp_new"))  // still valid
}

func TestColdStorage_FlushEmpty(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	// Flush with nothing pending — should not error
	require.NoError(t, cs.Flush())
}

func TestColdStorage_FlushRemovesEmptyFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}

	cs := NewColdStorage(dir, 0, comp, logger)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 1}})
	require.NoError(t, cs.Flush())

	// File exists
	_, err := os.Stat(cs.coldFilePath("default"))
	require.NoError(t, err)

	// Restore the only entry
	_, err = cs.Restore("default", "fp1")
	require.NoError(t, err)

	// Flush — file should be removed since it's empty
	require.NoError(t, cs.Flush())

	_, err = os.Stat(cs.coldFilePath("default"))
	assert.True(t, os.IsNotExist(err))
}

func TestColdStorage_RestoreIndex_NoDir(t *testing.T) {
	cs := NewColdStorage(filepath.Join(t.TempDir(), "nonexistent", "fingerprints"), 0, &testutil.MockCompressor{}, &testutil.MockLogger{})
	// Should create dir and succeed
	require.NoError(t, cs.RestoreIndex())
}

func TestColdStorage_RestoreIndex_EmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	require.NoError(t, os.MkdirAll(dir, 0755))

	cs := NewColdStorage(dir, 0, &testutil.MockCompressor{}, &testutil.MockLogger{})
	require.NoError(t, cs.RestoreIndex())
	assert.Empty(t, cs.index)
}

func TestColdStorage_CorruptFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	require.NoError(t, os.MkdirAll(dir, 0755))

	// Write a corrupt file (MockCompressor returns raw data, so write invalid JSON)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "default.cold.zst"), []byte("not json"), 0644))

	cs := NewColdStorage(dir, 0, &testutil.MockCompressor{}, &testutil.MockLogger{})
	require.NoError(t, cs.RestoreIndex())

	// Index should be empty — corrupt file is skipped
	assert.False(t, cs.Has("default", "fp1"))
}

func TestColdStorage_EvictOverwrite(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 1}})
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 99}})

	restored, err := cs.Restore("default", "fp1")
	require.NoError(t, err)
	assert.Equal(t, 99, restored[1].Views) // latest wins
}

func TestColdStorage_MultipleChannels(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	cs.Evict("ch1", "fp1", map[int]*models.StatRecord{1: {Views: 1}})
	cs.Evict("ch2", "fp1", map[int]*models.StatRecord{1: {Views: 2}})

	assert.True(t, cs.Has("ch1", "fp1"))
	assert.True(t, cs.Has("ch2", "fp1"))

	r1, _ := cs.Restore("ch1", "fp1")
	assert.Equal(t, 1, r1[1].Views)

	r2, _ := cs.Restore("ch2", "fp1")
	assert.Equal(t, 2, r2[1].Views)
}

func TestColdStorage_FlushMergesPendingWithExisting(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}

	cs := NewColdStorage(dir, 0, comp, logger)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 1}})
	require.NoError(t, cs.Flush())

	// Evict another fingerprint
	cs.Evict("default", "fp2", map[int]*models.StatRecord{2: {Views: 2}})
	require.NoError(t, cs.Flush())

	// Both should be on disk
	cs2 := NewColdStorage(dir, 0, comp, logger)
	require.NoError(t, cs2.RestoreIndex())

	assert.True(t, cs2.Has("default", "fp1"))
	assert.True(t, cs2.Has("default", "fp2"))
}

func TestColdStorage_ConcurrentAccess(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	var wg sync.WaitGroup

	// Concurrent evicts
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fp := "fp" + itoa(i)
			cs.Evict("default", fp, map[int]*models.StatRecord{1: {Views: i}})
		}(i)
	}

	// Concurrent Has checks
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cs.Has("default", "fp"+itoa(i))
		}(i)
	}

	wg.Wait()

	// Flush should succeed
	require.NoError(t, cs.Flush())
}

func TestColdStorage_ExtractChannelName(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	assert.Equal(t, "default", cs.extractChannelName("/some/path/default.cold.zst"))
	assert.Equal(t, "news", cs.extractChannelName("news.cold.zst"))
	assert.Equal(t, "sports.live", cs.extractChannelName("/dir/sports.live.cold.zst"))
}

func TestColdStorage_PendingCleanupOnRestore(t *testing.T) {
	cs := newTestColdStorage(t, 0)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 1}})

	_, err := cs.Restore("default", "fp1")
	require.NoError(t, err)

	// Pending channel map should be cleaned up
	cs.mu.RLock()
	_, hasPendingChannel := cs.pending["default"]
	cs.mu.RUnlock()
	assert.False(t, hasPendingChannel)
}

func TestColdStorage_FlushError_PreservesPending(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fingerprints")
	comp := &testutil.MockCompressor{
		CompressFn: func(b []byte) ([]byte, error) {
			return nil, fmt.Errorf("disk full")
		},
	}
	logger := &testutil.MockLogger{}

	cs := NewColdStorage(dir, 0, comp, logger)
	cs.Evict("default", "fp1", map[int]*models.StatRecord{1: {Views: 42}})

	// Flush fails due to compress error
	err := cs.Flush()
	require.Error(t, err)

	// Pending data must NOT be lost
	assert.True(t, cs.Has("default", "fp1"))
	cs.mu.RLock()
	_, stillPending := cs.pending["default"]["fp1"]
	cs.mu.RUnlock()
	assert.True(t, stillPending, "pending entry must survive flush failure")

	// Fix compressor — retry should succeed
	comp.CompressFn = nil
	require.NoError(t, cs.Flush())

	// Verify data is on disk
	cs2 := NewColdStorage(dir, 0, &testutil.MockCompressor{}, logger)
	require.NoError(t, cs2.RestoreIndex())
	assert.True(t, cs2.Has("default", "fp1"))

	restored, err := cs2.Restore("default", "fp1")
	require.NoError(t, err)
	assert.Equal(t, 42, restored[1].Views)
}

func TestColdStorage_Close_ClosesCompressor(t *testing.T) {
	closed := false
	comp := &closableCompressor{
		MockCompressor: testutil.MockCompressor{},
		closeFn:        func() { closed = true },
	}
	cs := NewColdStorage(t.TempDir(), 0, comp, &testutil.MockLogger{})
	cs.Close()
	assert.True(t, closed, "Close must call compressor.Close()")
}

// closableCompressor wraps MockCompressor with a trackable Close.
type closableCompressor struct {
	testutil.MockCompressor
	closeFn func()
}

func (c *closableCompressor) Close() {
	c.closeFn()
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
