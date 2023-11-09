package services

import (
	"go.uber.org/atomic"
	"ssd/internal/models"
)

var (
	Buffer1 struct {
		atomic.Bool
		Data []*models.InputStats
	}

	Buffer2 struct {
		atomic.Bool
		Data []*models.InputStats
	}

	statistic = &models.Statistic{
		Data: make(map[int]*models.StatRecord),
	}

	personalStats = &models.PersonalStats{
		Data: make(map[string]*models.Statistic),
	}
)

type StatisticServiceInterface interface {
	AddStats(data *models.InputStats)
	GetBuffer() []*models.InputStats
	GetNotActiveBuffer() []*models.InputStats
	SwitchBuffer()
	ClearNotActiveBuffer()
	AggregateStats()
	GetStatistic() map[int]*models.StatRecord
	PutStatistic(data map[int]*models.StatRecord)
	GetPersonalStatistic() map[string]*models.Statistic
	GetByFingerprint(fp string) map[int]*models.StatRecord
	PutPersonalStatistic(stats map[string]*models.Statistic)
}

type StatisticService struct {
}

func (ss *StatisticService) AddStats(data *models.InputStats) {
	if Buffer1.Load() {
		Buffer1.Data = append(Buffer1.Data, data)
	} else {
		Buffer2.Data = append(Buffer2.Data, data)
	}
}

func (ss *StatisticService) GetBuffer() []*models.InputStats {
	if Buffer1.Load() {
		return Buffer1.Data
	} else {
		return Buffer2.Data
	}
}

func (ss *StatisticService) SwitchBuffer() {
	Buffer1.Store(!Buffer1.Load())
	Buffer2.Store(!Buffer2.Load())
}

func (ss *StatisticService) ClearNotActiveBuffer() {
	if Buffer1.Load() {
		Buffer2.Data = make([]*models.InputStats, 0)
	} else {
		Buffer1.Data = make([]*models.InputStats, 0)
	}
}

func (ss *StatisticService) GetNotActiveBuffer() []*models.InputStats {
	if Buffer1.Load() {
		return Buffer2.Data
	} else {
		return Buffer1.Data
	}
}

func (ss *StatisticService) AggregateStats() {
	ss.SwitchBuffer()
	for _, v := range ss.GetNotActiveBuffer() {
		statistic.IncStats(v)
		personalStats.IncStats(v)
	}
	ss.ClearNotActiveBuffer()
}

func (ss *StatisticService) GetStatistic() map[int]*models.StatRecord {
	return statistic.GetData()
}

func (ss *StatisticService) PutStatistic(data map[int]*models.StatRecord) {
	statistic.PutData(data)
}

func (ss *StatisticService) PutPersonalStatistic(stats map[string]*models.Statistic) {
	personalStats.PutData(stats)
}

func (ss *StatisticService) GetPersonalStatistic() map[string]*models.Statistic {
	return personalStats.GetData()
}

func (ss *StatisticService) GetByFingerprint(fp string) map[int]*models.StatRecord {
	if val, ok := personalStats.Get(fp); ok {
		return val.GetData()
	}
	return nil

}

func NewStatisticService() StatisticServiceInterface {
	Buffer1.Data = make([]*models.InputStats, 0)
	Buffer2.Data = make([]*models.InputStats, 0)

	Buffer1.Store(true)
	Buffer2.Store(false)

	return &StatisticService{}
}
