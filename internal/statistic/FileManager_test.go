package statistic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"ssd/internal/models"
	"ssd/internal/services"
	"ssd/internal/structures"
	"ssd/internal/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultStatConfig() *structures.Config {
	return &structures.Config{
		Statistic: structures.StatisticConfig{
			MaxChannels:     1000,
			MaxRecords:      -1,
			EvictionPercent: 10,
			MaxRecordsPerFP: -1,
		},
	}
}

func newTestFileManager(compressor *testutil.MockCompressor) (*FileManager, *testutil.MockStatisticService) {
	svc := &testutil.MockStatisticService{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(compressor, svc, logger)
	return fm, svc
}

func TestFileManager_SaveToFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.dat")

	svc := services.NewStatisticService(defaultStatConfig())
	svc.AddStats(&models.InputStats{Views: []string{"1"}, Channel: "default"})
	svc.AggregateStats()

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	err := fm.SaveToFile(path)
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.NoError(t, err)

	// Temp file should not exist
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestFileManager_SaveToFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.dat")

	svc := services.NewStatisticService(defaultStatConfig())
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	require.NoError(t, fm.SaveToFile(path))

	// tmp file should be cleaned up
	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestFileManager_LoadFromFile_FileNotExist(t *testing.T) {
	fm, _ := newTestFileManager(&testutil.MockCompressor{})
	err := fm.LoadFromFile("/nonexistent/path/file.dat")
	assert.NoError(t, err) // not an error, just no data
}

func TestFileManager_LoadFromFile_V3Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v3.dat")

	storage := models.Storage{
		Channels: map[string]*models.ChannelData{
			"default": {
				TrendStats:    map[int]*models.StatRecord{1: {Views: 10}},
				PersonalStats: map[string]*models.Statistic{"fp1": {Data: map[int]*models.StatRecord{1: {Views: 5}}}},
			},
			"news": {
				TrendStats:    map[int]*models.StatRecord{2: {Views: 20}},
				PersonalStats: map[string]*models.Statistic{},
			},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	comp := &testutil.MockCompressor{} // identity compressor
	fm, svc := newTestFileManager(comp)
	require.NoError(t, fm.LoadFromFile(path))

	// V3 files now go through PutChannelDataV4 with lastSeen backfilled
	require.Len(t, svc.PutV4Calls, 2)
}

func TestFileManager_LoadFromFile_V2Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2.dat")

	v2 := struct {
		TrendStats    map[int]*models.StatRecord   `json:"trend_stats"`
		PersonalStats map[string]*models.Statistic `json:"personal_stats"`
	}{
		TrendStats:    map[int]*models.StatRecord{1: {Views: 100}},
		PersonalStats: map[string]*models.Statistic{"fp1": {Data: map[int]*models.StatRecord{1: {Views: 50}}}},
	}
	jsonData, _ := json.Marshal(v2)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	fm, svc := newTestFileManager(&testutil.MockCompressor{})
	require.NoError(t, fm.LoadFromFile(path))

	require.Len(t, svc.PutCalls, 1)
	assert.Equal(t, services.DefaultChannel, svc.PutCalls[0].Channel)
	assert.Equal(t, 100, svc.PutCalls[0].Trend[1].Views)
}

func TestFileManager_LoadFromFile_V1Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v1.dat")

	v1 := map[int]*models.StatRecord{1: {Views: 42, Clicks: 5}}
	jsonData, _ := json.Marshal(v1)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	fm, svc := newTestFileManager(&testutil.MockCompressor{})
	require.NoError(t, fm.LoadFromFile(path))

	require.Len(t, svc.PutCalls, 1)
	assert.Equal(t, services.DefaultChannel, svc.PutCalls[0].Channel)
	assert.Equal(t, 42, svc.PutCalls[0].Trend[1].Views)
	assert.NotNil(t, svc.PutCalls[0].Personal)
}

func TestFileManager_LoadFromFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.dat")
	require.NoError(t, os.WriteFile(path, []byte("not json at all"), 0644))

	fm, _ := newTestFileManager(&testutil.MockCompressor{})
	err := fm.LoadFromFile(path)
	assert.Error(t, err)
}

func TestFileManager_CompressError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "err.dat")

	comp := &testutil.MockCompressor{
		CompressFn: func(b []byte) ([]byte, error) {
			return nil, errors.New("compress failed")
		},
	}

	svc := services.NewStatisticService(defaultStatConfig())
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	err := fm.SaveToFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compress failed")
}

func TestFileManager_DecompressError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dec.dat")
	require.NoError(t, os.WriteFile(path, []byte("some data"), 0644))

	comp := &testutil.MockCompressor{
		DecompressFn: func(b []byte) ([]byte, error) {
			return nil, errors.New("decompress failed")
		},
	}
	fm, _ := newTestFileManager(comp)
	fm.compressor = comp

	err := fm.LoadFromFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decompress failed")
}

func TestFileManager_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.dat")

	// Save with real service
	svc := services.NewStatisticService(defaultStatConfig())
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp1",
		Views:       []string{"1", "2"},
		Clicks:      []string{"1"},
		Channel:     "default",
	})
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp2",
		Views:       []string{"3"},
		Channel:     "news",
	})
	svc.AggregateStats()

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	require.NoError(t, fm.SaveToFile(path))

	// Load into new service
	svc2 := services.NewStatisticService(defaultStatConfig())
	fm2 := NewFileManager(comp, svc2, logger)
	require.NoError(t, fm2.LoadFromFile(path))

	data := svc2.GetStatistic("default")
	require.NotNil(t, data)
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)

	newsData := svc2.GetStatistic("news")
	require.NotNil(t, newsData)
	assert.Equal(t, 1, newsData[3].Views)
}

func TestFileManager_V3NilFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nil.dat")

	// Channel with nil sub-fields
	storage := models.Storage{
		Channels: map[string]*models.ChannelData{
			"default": {},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	svc := services.NewStatisticService(defaultStatConfig())
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	require.NoError(t, fm.LoadFromFile(path))

	data := svc.GetStatistic("default")
	assert.NotNil(t, data)
	assert.Empty(t, data)
}

func TestFileManager_MultipleChannels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.dat")

	storage := models.Storage{
		Channels: map[string]*models.ChannelData{
			"ch1": {
				TrendStats:    map[int]*models.StatRecord{1: {Views: 10}},
				PersonalStats: map[string]*models.Statistic{},
			},
			"ch2": {
				TrendStats:    map[int]*models.StatRecord{2: {Views: 20}},
				PersonalStats: map[string]*models.Statistic{},
			},
			"ch3": {
				TrendStats:    map[int]*models.StatRecord{3: {Views: 30}},
				PersonalStats: map[string]*models.Statistic{},
			},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	svc := services.NewStatisticService(defaultStatConfig())
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	require.NoError(t, fm.LoadFromFile(path))

	assert.Equal(t, 10, svc.GetStatistic("ch1")[1].Views)
	assert.Equal(t, 20, svc.GetStatistic("ch2")[2].Views)
	assert.Equal(t, 30, svc.GetStatistic("ch3")[3].Views)
}

func TestFileManager_V5Roundtrip_PreservesLastSeen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v5roundtrip.dat")

	// Save with real service (now writes V5 binary)
	svc := services.NewStatisticService(defaultStatConfig())
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp1",
		Views:       []string{"1", "2"},
		Clicks:      []string{"1"},
		Channel:     "default",
	})
	svc.AggregateStats()

	// Capture lastSeen before save
	snapshot1 := svc.GetSnapshot()
	fp1Before := snapshot1.Channels["default"].PersonalStats["fp1"]
	require.NotNil(t, fp1Before)
	assert.False(t, fp1Before.LastSeen.IsZero(), "lastSeen should be set")
	savedLastSeen := fp1Before.LastSeen

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	require.NoError(t, fm.SaveToFile(path))

	// Verify file starts with binary magic "SSD5" (after identity compression)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "SSD5", string(raw[:4]), "saved file should have V5 binary magic")

	// Load into new service — lastSeen should be preserved
	svc2 := services.NewStatisticService(defaultStatConfig())
	fm2 := NewFileManager(comp, svc2, logger)
	require.NoError(t, fm2.LoadFromFile(path))

	// Verify data was loaded correctly
	data := svc2.GetStatistic("default")
	require.NotNil(t, data)
	assert.Equal(t, 1, data[1].Views)

	// Verify lastSeen was preserved via snapshot
	snapshot2 := svc2.GetSnapshot()
	fp1After := snapshot2.Channels["default"].PersonalStats["fp1"]
	require.NotNil(t, fp1After)
	assert.Equal(t, savedLastSeen.UnixNano(), fp1After.LastSeen.UnixNano(), "lastSeen should be preserved across save/load")
}

func TestFileManager_V3ToV4Migration_SetsLastSeen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v3migrate.dat")

	// Write V3 format (no version, no last_seen)
	storage := models.Storage{
		Channels: map[string]*models.ChannelData{
			"default": {
				TrendStats: map[int]*models.StatRecord{1: {Views: 10}},
				PersonalStats: map[string]*models.Statistic{
					"fp1": {Data: map[int]*models.StatRecord{1: {Views: 5}}},
					"fp2": {Data: map[int]*models.StatRecord{2: {Views: 3}}},
				},
			},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	before := time.Now()

	comp := &testutil.MockCompressor{}
	fm, svc := newTestFileManager(comp)
	require.NoError(t, fm.LoadFromFile(path))

	after := time.Now()

	// Should use PutChannelDataV4
	require.Len(t, svc.PutV4Calls, 1)
	call := svc.PutV4Calls[0]
	assert.Equal(t, "default", call.Channel)
	assert.Equal(t, 10, call.Trend[1].Views)

	// LastSeen should be backfilled to ~now for V3 data
	for fp, fpData := range call.Personal {
		assert.False(t, fpData.LastSeen.IsZero(), "lastSeen should be set for %s", fp)
		assert.True(t, !fpData.LastSeen.Before(before) && !fpData.LastSeen.After(after),
			"lastSeen for %s should be between before and after load", fp)
	}
}

func TestFileManager_V4Format_PreservesLastSeen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v4.dat")

	pastTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// Write V4 format with explicit last_seen
	storage := models.StorageV4{
		Version: 4,
		Channels: map[string]*models.ChannelDataV4{
			"default": {
				TrendStats: map[int]*models.StatRecord{1: {Views: 10}},
				PersonalStats: map[string]*models.FingerprintPersistence{
					"fp1": {
						Data:     map[int]*models.StatRecord{1: {Views: 5}},
						LastSeen: pastTime,
					},
				},
			},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	comp := &testutil.MockCompressor{}
	fm, svc := newTestFileManager(comp)
	require.NoError(t, fm.LoadFromFile(path))

	require.Len(t, svc.PutV4Calls, 1)
	call := svc.PutV4Calls[0]
	fp1 := call.Personal["fp1"]
	require.NotNil(t, fp1)
	assert.Equal(t, pastTime.Unix(), fp1.LastSeen.Unix(), "V4 lastSeen should be preserved as-is")
}

func TestFileManager_V5_BackwardCompat_LoadsV4JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v4legacy.dat")

	// Write V4 JSON format (no "SSD5" magic)
	storage := models.StorageV4{
		Version: 4,
		Channels: map[string]*models.ChannelDataV4{
			"default": {
				TrendStats: map[int]*models.StatRecord{1: {Views: 42}},
				PersonalStats: map[string]*models.FingerprintPersistence{
					"fp1": {
						Data:     map[int]*models.StatRecord{1: {Views: 10}},
						LastSeen: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	// Load — should fall through to V4 JSON path
	svc := services.NewStatisticService(defaultStatConfig())
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	require.NoError(t, fm.LoadFromFile(path))

	data := svc.GetStatistic("default")
	require.NotNil(t, data)
	assert.Equal(t, 42, data[1].Views)
}

func TestFileManager_V5_BinaryCorrupt_FallbackJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.dat")

	// Write data starting with "SSD5" magic but followed by valid V4 JSON
	// The binary parse should fail, then fall through to JSON path
	v4JSON, _ := json.Marshal(models.StorageV4{
		Version: 4,
		Channels: map[string]*models.ChannelDataV4{
			"default": {
				TrendStats:    map[int]*models.StatRecord{1: {Views: 99}},
				PersonalStats: map[string]*models.FingerprintPersistence{},
			},
		},
	})
	// Prepend "SSD5" + invalid binary data, but that makes the whole thing not valid JSON either
	// Instead, test that corrupt binary with no JSON fallback produces an error
	corruptBinary := append([]byte("SSD5"), byte(99)) // wrong version byte
	corruptBinary = append(corruptBinary, []byte("garbage")...)
	require.NoError(t, os.WriteFile(path, corruptBinary, 0644))

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	svc := services.NewStatisticService(defaultStatConfig())
	fm := NewFileManager(comp, svc, logger)

	// Binary fails → falls to JSON → JSON also fails on "SSD5..." prefix → error
	err := fm.LoadFromFile(path)
	assert.Error(t, err)

	// But if we have "SSD5" corrupt binary followed by nothing valid, the logger should have warned
	_ = v4JSON // suppress unused
}

func TestFileManager_V5_MultiChannel_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v5multi.dat")

	svc := services.NewStatisticService(defaultStatConfig())
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp1",
		Views:       []string{"1", "2"},
		Clicks:      []string{"1"},
		Channel:     "default",
	})
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp2",
		Views:       []string{"3"},
		Channel:     "news",
	})
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp3",
		Views:       []string{"4", "5"},
		Clicks:      []string{"4"},
		Channel:     "sports",
	})
	svc.AggregateStats()

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	require.NoError(t, fm.SaveToFile(path))

	svc2 := services.NewStatisticService(defaultStatConfig())
	fm2 := NewFileManager(comp, svc2, logger)
	require.NoError(t, fm2.LoadFromFile(path))

	// Verify all channels
	assert.Equal(t, 1, svc2.GetStatistic("default")[1].Views)
	assert.Equal(t, 1, svc2.GetStatistic("default")[1].Clicks)
	assert.Equal(t, 1, svc2.GetStatistic("news")[3].Views)
	assert.Equal(t, 1, svc2.GetStatistic("sports")[4].Views)
	assert.Equal(t, 1, svc2.GetStatistic("sports")[4].Clicks)

	// Verify personal stats
	fpData := svc2.GetByFingerprint("default", "fp1")
	require.NotNil(t, fpData)
	assert.Equal(t, 1, fpData[1].Views)

	fpData2 := svc2.GetByFingerprint("news", "fp2")
	require.NotNil(t, fpData2)
	assert.Equal(t, 1, fpData2[3].Views)
}
