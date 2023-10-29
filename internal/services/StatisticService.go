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
	}
	ss.ClearNotActiveBuffer()
}

func (ss *StatisticService) GetStatistic() map[int]*models.StatRecord {
	return statistic.GetData()
}

func (ss *StatisticService) PutStatistic(data map[int]*models.StatRecord) {
	statistic.PutData(data)
}

func NewStatisticService() StatisticServiceInterface {
	Buffer1.Data = make([]*models.InputStats, 0)
	Buffer2.Data = make([]*models.InputStats, 0)

	Buffer1.Store(true)
	Buffer2.Store(false)

	return &StatisticService{}
}
