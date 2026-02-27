package services

import (
	"fmt"
	"sort"
	"ssd/internal/models"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService() *StatisticService {
	return NewStatisticService().(*StatisticService)
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
	for i := 0; i < maxChannels-1; i++ {
		ch := ss.getOrCreateChannel(fmt.Sprintf("ch%d", i))
		require.NotNil(t, ch)
	}
	assert.Len(t, ss.channels, maxChannels)

	// Next one should return nil
	ch := ss.getOrCreateChannel("overflow")
	assert.Nil(t, ch)
}

func TestMaxChannels_AggregateSkipsOverflow(t *testing.T) {
	ss := newService()
	for i := 0; i < maxChannels-1; i++ {
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
