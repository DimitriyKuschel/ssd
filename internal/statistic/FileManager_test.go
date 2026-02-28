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

	require.Len(t, svc.PutCalls, 2)
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
