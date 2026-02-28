package services

import (
	"sort"
	"ssd/internal/models"
	"ssd/internal/structures"
	"sync"
)

const DefaultChannel = "default"

type StatisticServiceInterface interface {
	AddStats(data *models.InputStats)
	AggregateStats()
	GetStatistic(channel string) map[int]*models.StatRecord
	GetPersonalStatistic(channel string) map[string]*models.Statistic
	GetByFingerprint(channel, fp string) map[int]*models.StatRecord
	PutChannelData(channel string, trend map[int]*models.StatRecord, personal map[string]*models.Statistic)
	GetChannels() []string
	GetSnapshot() *models.Storage
	GetBufferSize() int
	GetRecordCount(channel string) int
}

type channelData struct {
	stats         *models.StatStore
	personalStats *models.PersonalStats
}

type StatisticService struct {
	mu              sync.Mutex
	activeIdx       int
	buffers         [2][]*models.InputStats
	prevBufSize     int
	chMu            sync.RWMutex
	channels        map[string]*channelData
	cachedChannels  []string
	maxChannels     int
	maxRecords      int
	evictionPercent int
}

func (ss *StatisticService) getOrCreateChannel(name string) *channelData {
	// Fast path: read lock for existing channels
	ss.chMu.RLock()
	if ch, ok := ss.channels[name]; ok {
		ss.chMu.RUnlock()
		return ch
	}
	ss.chMu.RUnlock()

	// Slow path: write lock with double-check
	ss.chMu.Lock()
	defer ss.chMu.Unlock()
	if ch, ok := ss.channels[name]; ok {
		return ch
	}
	if ss.maxChannels >= 0 && len(ss.channels) >= ss.maxChannels {
		return nil
	}
	ch := &channelData{
		stats: models.NewStatStore(ss.maxRecords, ss.evictionPercent),
		personalStats: &models.PersonalStats{
			Data: make(map[string]*models.Statistic),
		},
	}
	ss.channels[name] = ch
	ss.rebuildChannelCache()
	return ch
}

func (ss *StatisticService) rebuildChannelCache() {
	channels := make([]string, 0, len(ss.channels))
	for name := range ss.channels {
		channels = append(channels, name)
	}
	sort.Strings(channels)
	ss.cachedChannels = channels
}

func (ss *StatisticService) AddStats(data *models.InputStats) {
	ss.mu.Lock()
	idx := ss.activeIdx
	if ss.buffers[idx] == nil && ss.prevBufSize > 0 {
		ss.buffers[idx] = make([]*models.InputStats, 0, ss.prevBufSize)
	}
	ss.buffers[idx] = append(ss.buffers[idx], data)
	ss.mu.Unlock()
}

func (ss *StatisticService) AggregateStats() {
	ss.mu.Lock()
	ss.activeIdx = 1 - ss.activeIdx
	inactiveIdx := 1 - ss.activeIdx
	data := ss.buffers[inactiveIdx]
	ss.buffers[inactiveIdx] = nil
	if len(data) > 0 {
		ss.prevBufSize = len(data)
	}
	ss.mu.Unlock()

	for _, v := range data {
		chName := v.Channel
		if chName == "" {
			chName = DefaultChannel
		}
		ch := ss.getOrCreateChannel(chName)
		if ch == nil {
			continue
		}
		ch.stats.IncStats(v)
		ch.personalStats.IncStats(v)
	}
}

func (ss *StatisticService) GetStatistic(channel string) map[int]*models.StatRecord {
	ss.chMu.RLock()
	ch, ok := ss.channels[channel]
	ss.chMu.RUnlock()
	if ok {
		return ch.stats.GetData()
	}
	return nil
}

func (ss *StatisticService) GetPersonalStatistic(channel string) map[string]*models.Statistic {
	ss.chMu.RLock()
	ch, ok := ss.channels[channel]
	ss.chMu.RUnlock()
	if ok {
		return ch.personalStats.GetData()
	}
	return nil
}

func (ss *StatisticService) GetByFingerprint(channel, fp string) map[int]*models.StatRecord {
	ss.chMu.RLock()
	ch, ok := ss.channels[channel]
	ss.chMu.RUnlock()
	if ok {
		if val, ok := ch.personalStats.Get(fp); ok {
			return val.GetData()
		}
	}
	return nil
}

func (ss *StatisticService) PutChannelData(channel string, trend map[int]*models.StatRecord, personal map[string]*models.Statistic) {
	ch := ss.getOrCreateChannel(channel)
	if ch == nil {
		return
	}
	ch.stats.PutData(trend)
	ch.personalStats.PutData(personal)
}

func (ss *StatisticService) GetChannels() []string {
	ss.chMu.RLock()
	defer ss.chMu.RUnlock()
	return ss.cachedChannels
}

func (ss *StatisticService) GetSnapshot() *models.Storage {
	ss.chMu.RLock()
	defer ss.chMu.RUnlock()

	storage := &models.Storage{
		Channels: make(map[string]*models.ChannelData, len(ss.channels)),
	}
	for name, ch := range ss.channels {
		storage.Channels[name] = &models.ChannelData{
			TrendStats:    ch.stats.GetData(),
			PersonalStats: ch.personalStats.GetData(),
		}
	}
	return storage
}

func (ss *StatisticService) GetBufferSize() int {
	ss.mu.Lock()
	n := len(ss.buffers[ss.activeIdx])
	ss.mu.Unlock()
	return n
}

func (ss *StatisticService) GetRecordCount(channel string) int {
	ss.chMu.RLock()
	ch, ok := ss.channels[channel]
	ss.chMu.RUnlock()
	if ok {
		return ch.stats.Len()
	}
	return 0
}

func NewStatisticService(config *structures.Config) StatisticServiceInterface {
	maxChannels := config.Statistic.MaxChannels
	if maxChannels == 0 {
		maxChannels = 1000
	}
	maxRecords := config.Statistic.MaxRecords
	if maxRecords == 0 {
		maxRecords = -1
	}
	evictionPercent := config.Statistic.EvictionPercent
	if evictionPercent <= 0 {
		evictionPercent = 10
	}

	ss := &StatisticService{
		activeIdx:       0,
		channels:        make(map[string]*channelData),
		maxChannels:     maxChannels,
		maxRecords:      maxRecords,
		evictionPercent: evictionPercent,
	}
	ss.getOrCreateChannel(DefaultChannel)
	return ss
}
