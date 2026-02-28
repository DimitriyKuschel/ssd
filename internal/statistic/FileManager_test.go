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

func TestFileManager_V4Roundtrip_PreservesLastSeen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v4roundtrip.dat")

	// Save with real service
	svc := services.NewStatisticService(defaultStatConfig())
	svc.AddStats(&models.InputStats{
		Fingerprint: "fp1",
		Views:       []string{"1", "2"},
		Clicks:      []string{"1"},
		Channel:     "default",
	})
	svc.AggregateStats()

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	require.NoError(t, fm.SaveToFile(path))

	// Verify saved data is V4 with version field
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var saved models.StorageV4
	require.NoError(t, json.Unmarshal(raw, &saved))
	assert.Equal(t, 4, saved.Version)
	assert.NotNil(t, saved.Channels["default"])
	fp1 := saved.Channels["default"].PersonalStats["fp1"]
	require.NotNil(t, fp1)
	assert.False(t, fp1.LastSeen.IsZero(), "lastSeen should be set")
	savedLastSeen := fp1.LastSeen

	// Load into new service — lastSeen should be preserved
	svc2 := services.NewStatisticService(defaultStatConfig())
	fm2 := NewFileManager(comp, svc2, logger)
	require.NoError(t, fm2.LoadFromFile(path))

	// Verify data was loaded correctly
	data := svc2.GetStatistic("default")
	require.NotNil(t, data)
	assert.Equal(t, 1, data[1].Views)

	// Verify lastSeen was preserved by saving again and checking
	path2 := filepath.Join(dir, "v4roundtrip2.dat")
	fm3 := NewFileManager(comp, svc2, logger)
	require.NoError(t, fm3.SaveToFile(path2))

	raw2, err := os.ReadFile(path2)
	require.NoError(t, err)
	var saved2 models.StorageV4
	require.NoError(t, json.Unmarshal(raw2, &saved2))
	fp1Again := saved2.Channels["default"].PersonalStats["fp1"]
	require.NotNil(t, fp1Again)
	assert.Equal(t, savedLastSeen.Unix(), fp1Again.LastSeen.Unix(), "lastSeen should be preserved across save/load")
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
