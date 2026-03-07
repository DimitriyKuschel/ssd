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
			MaxRecordsPerFP: -1,
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

func testConfigWithTTL(filePath string, fpTTL, coldTTL time.Duration) *structures.Config {
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
			MaxRecordsPerFP: -1,
			FingerprintTTL:  fpTTL,
			ColdTTL:         coldTTL,
		},
	}
}

func TestScheduler_ColdStorage_CreatedWhenTTLSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.dat")
	conf := testConfigWithTTL(path, 1*time.Hour, 0)

	svc := services.NewStatisticService(conf)
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{}).(*Scheduler)
	assert.NotNil(t, s.cold)
}

func TestScheduler_ColdStorage_NotCreatedWhenNoTTL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.dat")
	conf := testConfigWithTTL(path, 0, 0)

	svc := services.NewStatisticService(conf)
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{}).(*Scheduler)
	assert.Nil(t, s.cold)
}

func TestScheduler_ColdStorage_EvictFlushAndRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.dat")
	conf := testConfigWithTTL(path, 1*time.Hour, 0)

	svc := services.NewStatisticService(conf)
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{}).(*Scheduler)
	require.NotNil(t, s.cold)

	// Evict a fingerprint through cold storage directly
	fpData := map[int]*models.StatRecord{
		1: {Views: 10, Clicks: 3},
		2: {Views: 1},
	}
	s.cold.Evict("default", "fp1", fpData)
	assert.True(t, s.cold.Has("default", "fp1"))

	// Persist flushes cold storage
	require.NoError(t, s.Persist())

	// Create new scheduler with same dir, restore cold index
	svc2 := services.NewStatisticService(conf)
	fm2 := NewFileManager(comp, svc2, logger)
	s2 := NewScheduler(conf, logger, svc2, fm2, &testutil.MockMetrics{}).(*Scheduler)
	require.NotNil(t, s2.cold)
	require.NoError(t, s2.Restore())

	assert.True(t, s2.cold.Has("default", "fp1"))

	// Restore from cold
	restored, err := s2.cold.Restore("default", "fp1")
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, 10, restored[1].Views)
	assert.Equal(t, 3, restored[1].Clicks)
	assert.Equal(t, 1, restored[2].Views)
}

func TestScheduler_Persist_FlushesColdstorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.dat")
	conf := testConfigWithTTL(path, 1*time.Hour, 0)

	svc := services.NewStatisticService(conf)
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	s := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{}).(*Scheduler)
	require.NotNil(t, s.cold)

	// Evict directly into cold storage
	s.cold.Evict("default", "fp_test", map[int]*models.StatRecord{1: {Views: 99}})

	// Persist should flush cold storage too
	require.NoError(t, s.Persist())

	// Cold file should exist on disk
	_, err := os.Stat(s.cold.coldFilePath("default"))
	assert.NoError(t, err)
}

func TestScheduler_Restore_RestoresColdIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.dat")
	conf := testConfigWithTTL(path, 1*time.Hour, 0)

	svc := services.NewStatisticService(conf)
	comp := &testutil.MockCompressor{}
	logger := &testutil.MockLogger{}
	fm := NewFileManager(comp, svc, logger)

	// Create first scheduler, evict data, persist (flushes cold)
	s1 := NewScheduler(conf, logger, svc, fm, &testutil.MockMetrics{}).(*Scheduler)
	require.NotNil(t, s1.cold)
	s1.cold.Evict("default", "fp_cold", map[int]*models.StatRecord{1: {Views: 50}})
	require.NoError(t, s1.Persist())

	// Create second scheduler from scratch — Restore should pick up cold index
	svc2 := services.NewStatisticService(conf)
	fm2 := NewFileManager(comp, svc2, logger)
	s2 := NewScheduler(conf, logger, svc2, fm2, &testutil.MockMetrics{}).(*Scheduler)
	require.NotNil(t, s2.cold)
	require.NoError(t, s2.Restore())

	assert.True(t, s2.cold.Has("default", "fp_cold"))
}
