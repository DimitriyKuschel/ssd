package models

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPSS() *PersonalStatStore {
	return NewPersonalStatStore("default", -1, -1, 10, 0, nil)
}

func TestPSS_IncStats_NewFingerprint(t *testing.T) {
	ps := newPSS()
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1", "2"}})

	assert.Equal(t, 1, ps.Len())
	val, ok := ps.Get("fp1")
	require.True(t, ok)
	assert.Len(t, val.Data, 2)
}

func TestPSS_IncStats_ExistingFingerprint(t *testing.T) {
	ps := newPSS()
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1"}})
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1", "2"}})

	val, _ := ps.Get("fp1")
	assert.Len(t, val.Data, 2)
	assert.Equal(t, 2, val.Data[1].Views) // viewed twice
}

func TestPSS_IncStats_Nil(t *testing.T) {
	ps := newPSS()
	ps.IncStats(nil)
	assert.Equal(t, 0, ps.Len())
}

func TestPSS_GetMissing(t *testing.T) {
	ps := newPSS()
	val, ok := ps.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestPSS_MaxFingerprints(t *testing.T) {
	ps := NewPersonalStatStore("default", 10, -1, 10, 0, nil)

	for i := 0; i < 10; i++ {
		ps.IncStats(&InputStats{Fingerprint: fmt.Sprintf("fp%d", i), Views: []string{"1"}})
	}
	assert.Equal(t, 10, ps.Len())

	// Overflow — should be rejected
	ps.IncStats(&InputStats{Fingerprint: "overflow", Views: []string{"1"}})
	_, ok := ps.Get("overflow")
	assert.False(t, ok)
	assert.Equal(t, 10, ps.Len())
}

func TestPSS_MaxFingerprints_ExistingStillWorks(t *testing.T) {
	ps := NewPersonalStatStore("default", 10, -1, 10, 0, nil)

	for i := 0; i < 10; i++ {
		ps.IncStats(&InputStats{Fingerprint: fmt.Sprintf("fp%d", i), Views: []string{"1"}})
	}

	// Existing fingerprint should still get updates
	ps.IncStats(&InputStats{Fingerprint: "fp0", Views: []string{"1"}})
	val, ok := ps.Get("fp0")
	require.True(t, ok)
	assert.Equal(t, 2, val.Data[1].Views) // viewed twice
}

func TestPSS_GetData_DeepCopy(t *testing.T) {
	ps := newPSS()
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1"}})

	copied := ps.GetData()
	copied["fp1"].Data[1].Views = 999

	original, _ := ps.Get("fp1")
	assert.Equal(t, 1, original.Data[1].Views)
}

func TestPSS_PutData(t *testing.T) {
	ps := newPSS()
	ps.IncStats(&InputStats{Fingerprint: "old", Views: []string{"1"}})

	newData := map[string]*Statistic{
		"new": {Data: map[int]*StatRecord{1: {Views: 10}}},
	}
	ps.PutData(newData)

	assert.Equal(t, 1, ps.Len())
	_, ok := ps.Get("old")
	assert.False(t, ok)
	val, ok := ps.Get("new")
	require.True(t, ok)
	assert.Equal(t, 10, val.Data[1].Views)
}

func TestPSS_PutData_ConvertsToBitmaps(t *testing.T) {
	ps := newPSS()

	// Views=1, Clicks=0 should be bitmap-only (no counts entry)
	data := map[string]*Statistic{
		"fp1": {Data: map[int]*StatRecord{
			1: {Views: 1, Clicks: 0},
			2: {Views: 5, Clicks: 2, Ftr: 1},
		}},
	}
	ps.PutData(data)

	val, ok := ps.Get("fp1")
	require.True(t, ok)
	assert.Equal(t, 1, val.Data[1].Views)
	assert.Equal(t, 5, val.Data[2].Views)
	assert.Equal(t, 2, val.Data[2].Clicks)
	assert.Equal(t, 1, val.Data[2].Ftr)

	// Internal: ID 1 should be bitmap-only, ID 2 in counts
	ps.mu.RLock()
	fp := ps.fingerprints["fp1"]
	ps.mu.RUnlock()
	_, hasCounts1 := fp.counts[1]
	assert.False(t, hasCounts1) // Views: 1 is default, not in counts
	_, hasCounts2 := fp.counts[2]
	assert.True(t, hasCounts2)
}

func TestPSS_EvictExpired_NoTTL(t *testing.T) {
	ps := newPSS() // TTL = 0 -> disabled
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1"}})

	ps.EvictExpired(time.Now().Add(24 * time.Hour))
	assert.Equal(t, 1, ps.Len()) // should not evict
}

func TestPSS_EvictExpired_WithTTL(t *testing.T) {
	ps := NewPersonalStatStore("default", -1, -1, 10, 1*time.Hour, nil)

	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1"}})
	ps.IncStats(&InputStats{Fingerprint: "fp2", Views: []string{"2"}})

	// Set fp1's lastSeen to 2 hours ago
	ps.mu.Lock()
	ps.fingerprints["fp1"].lastSeen = time.Now().Add(-2 * time.Hour)
	ps.mu.Unlock()

	ps.EvictExpired(time.Now())

	assert.Equal(t, 1, ps.Len())
	_, ok := ps.Get("fp1")
	assert.False(t, ok) // expired
	_, ok = ps.Get("fp2")
	assert.True(t, ok) // still active
}

func TestPSS_EvictExpired_WithColdStorage(t *testing.T) {
	cold := &mockColdStorage{}
	ps := NewPersonalStatStore("default", -1, -1, 10, 1*time.Hour, cold)

	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1", "2"}})

	// Expire fp1
	ps.mu.Lock()
	ps.fingerprints["fp1"].lastSeen = time.Now().Add(-2 * time.Hour)
	ps.mu.Unlock()

	ps.EvictExpired(time.Now())

	assert.Equal(t, 0, ps.Len())
	// Data should be sent to cold storage
	require.Len(t, cold.evicted, 1)
	assert.Equal(t, "fp1", cold.evicted[0].fingerprint)
	assert.Equal(t, "default", cold.evicted[0].channel)
	assert.Len(t, cold.evicted[0].data, 2)
}

func TestPSS_ColdStorageRestore(t *testing.T) {
	coldData := map[int]*StatRecord{
		1: {Views: 10, Clicks: 3},
		2: {Views: 1},
	}
	cold := &mockColdStorage{
		hasMap: map[string]bool{"default:fp_cold": true},
		restoreData: map[string]map[int]*StatRecord{
			"default:fp_cold": coldData,
		},
	}
	ps := NewPersonalStatStore("default", -1, -1, 10, 0, cold)

	// IncStats for a fingerprint in cold storage
	ps.IncStats(&InputStats{Fingerprint: "fp_cold", Views: []string{"1"}})

	val, ok := ps.Get("fp_cold")
	require.True(t, ok)
	assert.Equal(t, 11, val.Data[1].Views) // 10 + 1
	assert.Equal(t, 3, val.Data[1].Clicks)
	assert.Equal(t, 1, val.Data[2].Views) // untouched
}

func TestPSS_MaxRecordsPerFP(t *testing.T) {
	ps := NewPersonalStatStore("default", -1, 5, 40, 0, nil) // max 5 IDs per FP

	// Add 5 views with different IDs
	for i := 0; i < 5; i++ {
		ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{itoa(i)}})
	}
	// Add one more — triggers eviction within fingerprint
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"99"}})

	val, _ := ps.Get("fp1")
	assert.LessOrEqual(t, len(val.Data), 4) // 5 - 40% + 1 = 4
	assert.Contains(t, val.Data, 99)        // new one exists
}

func TestPSS_ConcurrentAccess(t *testing.T) {
	ps := newPSS()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ps.IncStats(&InputStats{
				Fingerprint: fmt.Sprintf("fp%d", i%10),
				Views:       []string{"1"},
			})
		}(i)
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ps.GetData()
		}()
	}
	wg.Wait()

	assert.Greater(t, ps.Len(), 0)
	assert.LessOrEqual(t, ps.Len(), 10)
}

// mockColdStorage is a test-only implementation of ColdStorageInterface.
type mockColdStorage struct {
	mu          sync.Mutex
	evicted     []coldEvictCall
	hasMap      map[string]bool
	restoreData map[string]map[int]*StatRecord
}

type coldEvictCall struct {
	channel     string
	fingerprint string
	data        map[int]*StatRecord
}

func (m *mockColdStorage) Has(channel, fingerprint string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hasMap == nil {
		return false
	}
	return m.hasMap[channel+":"+fingerprint]
}

func (m *mockColdStorage) Evict(channel, fingerprint string, data map[int]*StatRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evicted = append(m.evicted, coldEvictCall{channel: channel, fingerprint: fingerprint, data: data})
}

func (m *mockColdStorage) Restore(channel, fingerprint string) (map[int]*StatRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := channel + ":" + fingerprint
	if m.restoreData != nil {
		if data, ok := m.restoreData[key]; ok {
			delete(m.hasMap, key)
			return data, nil
		}
	}
	return nil, nil
}
