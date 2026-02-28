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

func testConfig(filePath string) *structures.Config {
	return &structures.Config{
		Persistence: structures.Persistence{
			FilePath:     filePath,
			SaveInterval: 1 * time.Second,
		},
		Statistic: structures.StatisticConfig{
			Interval:        1 * time.Second,
			MaxChannels:     1000,
			MaxRecords:      -1,
			EvictionPercent: 10,
		},
	}
}

func TestScheduler_Restore_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "restore.dat")

	storage := models.Storage{
		Channels: map[string]*models.ChannelData{
			"default": {
				TrendStats:    map[int]*models.StatRecord{1: {Views: 42}},
				PersonalStats: map[string]*models.Statistic{},
			},
		},
	}
	jsonData, _ := json.Marshal(storage)
	require.NoError(t, os.WriteFile(path, jsonData, 0644))

	svc := services.NewStatisticService(testConfig(""))
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig(path)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	require.NoError(t, s.Restore())

	data := svc.GetStatistic("default")
	assert.Equal(t, 42, data[1].Views)
}

func TestScheduler_Restore_FileNotExist(t *testing.T) {
	svc := services.NewStatisticService(testConfig(""))
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig("/nonexistent/file.dat")

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	err := s.Restore()
	assert.NoError(t, err)
}

func TestScheduler_Restore_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.dat")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	svc := services.NewStatisticService(testConfig(""))
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig(path)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	err := s.Restore()
	assert.Error(t, err)
}

func TestScheduler_Persist_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.dat")

	svc := services.NewStatisticService(testConfig(""))
	svc.AddStats(&models.InputStats{Views: []string{"1"}, Channel: "default"})
	svc.AggregateStats()

	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig(path)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	require.NoError(t, s.Persist())

	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestScheduler_Persist_WriteError(t *testing.T) {
	comp := &testutil.MockCompressor{
		CompressFn: func(b []byte) ([]byte, error) {
			return nil, errors.New("compress error")
		},
	}
	svc := services.NewStatisticService(testConfig(""))
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig("/tmp/test.dat")

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	err := s.Persist()
	assert.Error(t, err)
}

func TestScheduler_StopNilCron(t *testing.T) {
	svc := services.NewStatisticService(testConfig(""))
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig("/tmp/test.dat")

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	// Should not panic with nil cron
	s.Stop()
}

func TestScheduler_InitAndStop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lifecycle.dat")

	svc := services.NewStatisticService(testConfig(""))
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)
	conf := testConfig(path)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{})
	s.Init()
	// Give the cron a moment to start
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}
