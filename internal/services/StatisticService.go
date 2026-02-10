package services

import (
	"ssd/internal/models"
	"sync"
)

type StatisticServiceInterface interface {
	AddStats(data *models.InputStats)
	AggregateStats()
	GetStatistic() map[int]*models.StatRecord
	PutStatistic(data map[int]*models.StatRecord)
	GetPersonalStatistic() map[string]*models.Statistic
	GetByFingerprint(fp string) map[int]*models.StatRecord
	PutPersonalStatistic(stats map[string]*models.Statistic)
}

type StatisticService struct {
	mu            sync.Mutex
	activeIdx     int
	buffers       [2][]*models.InputStats
	statistic     *models.Statistic
	personalStats *models.PersonalStats
}

func (ss *StatisticService) AddStats(data *models.InputStats) {
	ss.mu.Lock()
	idx := ss.activeIdx
	ss.buffers[idx] = append(ss.buffers[idx], data)
	ss.mu.Unlock()
}

func (ss *StatisticService) AggregateStats() {
	ss.mu.Lock()
	ss.activeIdx = 1 - ss.activeIdx
	inactiveIdx := 1 - ss.activeIdx
	data := ss.buffers[inactiveIdx]
	ss.buffers[inactiveIdx] = nil
	ss.mu.Unlock()

	for _, v := range data {
		ss.statistic.IncStats(v)
		ss.personalStats.IncStats(v)
	}
}

func (ss *StatisticService) GetStatistic() map[int]*models.StatRecord {
	return ss.statistic.GetData()
}

func (ss *StatisticService) PutStatistic(data map[int]*models.StatRecord) {
	ss.statistic.PutData(data)
}

func (ss *StatisticService) PutPersonalStatistic(stats map[string]*models.Statistic) {
	ss.personalStats.PutData(stats)
}

func (ss *StatisticService) GetPersonalStatistic() map[string]*models.Statistic {
	return ss.personalStats.GetData()
}

func (ss *StatisticService) GetByFingerprint(fp string) map[int]*models.StatRecord {
	if val, ok := ss.personalStats.Get(fp); ok {
		return val.GetData()
	}
	return nil
}

func NewStatisticService() StatisticServiceInterface {
	return &StatisticService{
		activeIdx: 0,
		statistic: &models.Statistic{
			Data: make(map[int]*models.StatRecord),
		},
		personalStats: &models.PersonalStats{
			Data: make(map[string]*models.Statistic),
		},
	}
}
