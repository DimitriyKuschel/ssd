package services

import (
	"bytes"
	"fmt"
	"sort"
	"ssd/internal/models"
	"ssd/internal/structures"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *structures.Config {
	return &structures.Config{
		Statistic: structures.StatisticConfig{
			MaxChannels:     1000,
			MaxRecords:      -1,
			EvictionPercent: 10,
			MaxRecordsPerFP: -1,
			FingerprintTTL:  0,
		},
	}
}

func newService() *StatisticService {
	return NewStatisticService(testConfig()).(*StatisticService)
}

func TestNewStatisticService_DefaultChannel(t *testing.T) {
	ss := newService()
	channels := ss.GetChannels()
	assert.Equal(t, []string{DefaultChannel}, channels)
}

func TestAddStats_BuffersSingleItem(t *testing.T) {
	ss := newService()
	input := &models.InputStats{Views: []string{"1"}, Channel: DefaultChannel}
	ss.AddStats(input)

	assert.Len(t, ss.buffers[ss.activeIdx], 1)
}

func TestAddStats_MultipleItems(t *testing.T) {
	ss := newService()
	for i := 0; i < 5; i++ {
		ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: DefaultChannel})
	}
	assert.Len(t, ss.buffers[ss.activeIdx], 5)
}

func TestAggregateStats_SwapsBuffers(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: DefaultChannel})

	initialIdx := ss.activeIdx
	ss.AggregateStats()

	assert.NotEqual(t, initialIdx, ss.activeIdx)
	// Inactive buffer should be cleared
	inactiveIdx := 1 - ss.activeIdx
	assert.Nil(t, ss.buffers[inactiveIdx])
}

func TestAggregateStats_ProcessesData(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Views: []string{"1", "2"}, Clicks: []string{"1"}, Channel: DefaultChannel})
	ss.AggregateStats()

	data := ss.GetStatistic(DefaultChannel)
	require.NotNil(t, data)
	assert.Equal(t, 2, len(data))
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
	assert.Equal(t, 1, data[2].Views)
}

func TestAggregateStats_EmptyChannelDefaultsToDefault(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: ""})
	ss.AggregateStats()

	data := ss.GetStatistic(DefaultChannel)
	require.NotNil(t, data)
	assert.Equal(t, 1, data[1].Views)
}

func TestAggregateStats_CustomChannel(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: "news"})
	ss.AggregateStats()

	data := ss.GetStatistic("news")
	require.NotNil(t, data)
	assert.Equal(t, 1, data[1].Views)

	defData := ss.GetStatistic(DefaultChannel)
	assert.Empty(t, defData)
}

func TestGetStatistic_NonexistentChannel(t *testing.T) {
	ss := newService()
	data := ss.GetStatistic("nonexistent")
	assert.Nil(t, data)
}

func TestGetPersonalStatistic(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Fingerprint: "fp1", Views: []string{"1"}, Channel: DefaultChannel})
	ss.AggregateStats()

	data := ss.GetPersonalStatistic(DefaultChannel)
	require.NotNil(t, data)
	assert.Contains(t, data, "fp1")
}

func TestGetPersonalStatistic_NonexistentChannel(t *testing.T) {
	ss := newService()
	data := ss.GetPersonalStatistic("nonexistent")
	assert.Nil(t, data)
}

func TestGetByFingerprint(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Fingerprint: "fp1", Views: []string{"1", "2"}, Channel: DefaultChannel})
	ss.AggregateStats()

	data := ss.GetByFingerprint(DefaultChannel, "fp1")
	require.NotNil(t, data)
	assert.Len(t, data, 2)
}

func TestGetByFingerprint_NonexistentFP(t *testing.T) {
	ss := newService()
	data := ss.GetByFingerprint(DefaultChannel, "nonexistent")
	assert.Nil(t, data)
}

func TestGetByFingerprint_NonexistentChannel(t *testing.T) {
	ss := newService()
	data := ss.GetByFingerprint("nonexistent", "fp1")
	assert.Nil(t, data)
}

func TestPutChannelData(t *testing.T) {
	ss := newService()
	trend := map[int]*models.StatRecord{1: {Views: 100}}
	personal := map[string]*models.Statistic{
		"fp1": {Data: map[int]*models.StatRecord{1: {Views: 50}}},
	}
	ss.PutChannelData("restored", trend, personal)

	data := ss.GetStatistic("restored")
	require.NotNil(t, data)
	assert.Equal(t, 100, data[1].Views)

	pData := ss.GetByFingerprint("restored", "fp1")
	require.NotNil(t, pData)
	assert.Equal(t, 50, pData[1].Views)
}

func TestGetChannels_Sorted(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: "zebra"})
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: "alpha"})
	ss.AggregateStats()

	channels := ss.GetChannels()
	assert.True(t, sort.StringsAreSorted(channels))
	assert.Contains(t, channels, "alpha")
	assert.Contains(t, channels, "zebra")
	assert.Contains(t, channels, DefaultChannel)
}

func TestMaxChannels(t *testing.T) {
	ss := newService()
	// Default channel is already created, so we can create maxChannels-1 more
	for i := 0; i < ss.maxChannels-1; i++ {
		ch := ss.getOrCreateChannel(fmt.Sprintf("ch%d", i))
		require.NotNil(t, ch)
	}
	assert.Len(t, ss.channels, ss.maxChannels)

	// Next one should return nil
	ch := ss.getOrCreateChannel("overflow")
	assert.Nil(t, ch)
}

func TestMaxChannels_AggregateSkipsOverflow(t *testing.T) {
	ss := newService()
	for i := 0; i < ss.maxChannels-1; i++ {
		ss.getOrCreateChannel(fmt.Sprintf("ch%d", i))
	}

	// This should not panic, the data is just silently dropped
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Channel: "overflow"})
	ss.AggregateStats()

	data := ss.GetStatistic("overflow")
	assert.Nil(t, data)
}

func TestConcurrent_AddAndAggregate(t *testing.T) {
	ss := newService()
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ss.AddStats(&models.InputStats{
				Fingerprint: fmt.Sprintf("fp%d", i%5),
				Views:       []string{"1", "2"},
				Clicks:      []string{"1"},
				Channel:     DefaultChannel,
			})
		}(i)
	}

	// Aggregators
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ss.AggregateStats()
		}()
	}

	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ss.GetStatistic(DefaultChannel)
			ss.GetChannels()
		}()
	}

	wg.Wait()
}

func TestGetSnapshot(t *testing.T) {
	ss := newService()
	ss.AddStats(&models.InputStats{Views: []string{"1"}, Clicks: []string{"1"}, Fingerprint: "fp1", Channel: DefaultChannel})
	ss.AggregateStats()

	snapshot := ss.GetSnapshot()
	require.NotNil(t, snapshot)
	assert.Equal(t, 4, snapshot.Version)
	require.Contains(t, snapshot.Channels, DefaultChannel)
	assert.Equal(t, 1, snapshot.Channels[DefaultChannel].TrendStats[1].Views)

	// PersonalStats should be FingerprintPersistence with lastSeen
	fp1 := snapshot.Channels[DefaultChannel].PersonalStats["fp1"]
	require.NotNil(t, fp1)
	assert.Equal(t, 1, fp1.Data[1].Views)
	assert.False(t, fp1.LastSeen.IsZero())
}

func TestPutChannelDataV4(t *testing.T) {
	ss := newService()
	pastTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	trend := map[int]*models.StatRecord{1: {Views: 100}}
	personal := map[string]*models.FingerprintPersistence{
		"fp1": {
			Data:     map[int]*models.StatRecord{1: {Views: 50}},
			LastSeen: pastTime,
		},
	}
	ss.PutChannelDataV4("restored", trend, personal)

	data := ss.GetStatistic("restored")
	require.NotNil(t, data)
	assert.Equal(t, 100, data[1].Views)

	pData := ss.GetByFingerprint("restored", "fp1")
	require.NotNil(t, pData)
	assert.Equal(t, 50, pData[1].Views)

	// Verify lastSeen was preserved via snapshot
	snapshot := ss.GetSnapshot()
	fp1 := snapshot.Channels["restored"].PersonalStats["fp1"]
	require.NotNil(t, fp1)
	assert.Equal(t, pastTime.Unix(), fp1.LastSeen.Unix())
}

func TestGetBufferSize(t *testing.T) {
	ss := newService()
	assert.Equal(t, 0, ss.GetBufferSize())
	ss.AddStats(&models.InputStats{Views: []string{"1"}})
	assert.Equal(t, 1, ss.GetBufferSize())
}

func TestGetRecordCount(t *testing.T) {
	ss := newService()
	assert.Equal(t, 0, ss.GetRecordCount(DefaultChannel))
	ss.AddStats(&models.InputStats{Views: []string{"1", "2", "3"}, Channel: DefaultChannel})
	ss.AggregateStats()
	assert.Equal(t, 3, ss.GetRecordCount(DefaultChannel))
}

func TestWriteBinarySnapshot_Roundtrip(t *testing.T) {
	ss := newService()

	// Add data to multiple channels
	ss.AddStats(&models.InputStats{
		Fingerprint: "fp1",
		Views:       []string{"1", "2"},
		Clicks:      []string{"1"},
		Channel:     "default",
	})
	ss.AddStats(&models.InputStats{
		Fingerprint: "fp2",
		Views:       []string{"3"},
		Channel:     "news",
	})
	ss.AggregateStats()

	// Capture lastSeen before write
	snap1 := ss.GetSnapshot()
	fp1LastSeen := snap1.Channels["default"].PersonalStats["fp1"].LastSeen

	// Write binary snapshot
	var buf bytes.Buffer
	require.NoError(t, ss.WriteBinarySnapshot(&buf))

	// Read into new service
	ss2 := newService()
	require.NoError(t, ss2.ReadBinarySnapshot(bytes.NewReader(buf.Bytes())))

	// Verify channels exist
	channels := ss2.GetChannels()
	assert.Contains(t, channels, "default")
	assert.Contains(t, channels, "news")

	// Verify trend data
	data := ss2.GetStatistic("default")
	require.NotNil(t, data)
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
	assert.Equal(t, 1, data[2].Views)

	newsData := ss2.GetStatistic("news")
	require.NotNil(t, newsData)
	assert.Equal(t, 1, newsData[3].Views)

	// Verify personal data
	fpData := ss2.GetByFingerprint("default", "fp1")
	require.NotNil(t, fpData)
	assert.Equal(t, 1, fpData[1].Views)

	// Verify lastSeen preserved
	snap2 := ss2.GetSnapshot()
	fp1After := snap2.Channels["default"].PersonalStats["fp1"]
	require.NotNil(t, fp1After)
	assert.Equal(t, fp1LastSeen.UnixNano(), fp1After.LastSeen.UnixNano())
}

func TestWriteBinarySnapshot_Empty(t *testing.T) {
	ss := newService()

	var buf bytes.Buffer
	require.NoError(t, ss.WriteBinarySnapshot(&buf))

	ss2 := newService()
	require.NoError(t, ss2.ReadBinarySnapshot(bytes.NewReader(buf.Bytes())))

	data := ss2.GetStatistic("default")
	assert.Empty(t, data)
}

func TestReadBinarySnapshot_InvalidMagic(t *testing.T) {
	ss := newService()
	err := ss.ReadBinarySnapshot(bytes.NewReader([]byte("XXXX")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid binary magic")
}

func TestReadBinarySnapshot_UnsupportedVersion(t *testing.T) {
	ss := newService()
	data := []byte{'S', 'S', 'D', '5', 99} // version 99
	err := ss.ReadBinarySnapshot(bytes.NewReader(data))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported binary version")
}
