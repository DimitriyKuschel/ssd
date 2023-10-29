package statistic

import (
	"github.com/roylee0704/gron"
	"ssd/internal/providers"
	"ssd/internal/services"
	"ssd/internal/statistic/interfaces"
	"ssd/internal/structures"

	"time"
)

type Scheduler struct {
	config      *structures.Config
	logger      providers.Logger
	service     services.StatisticServiceInterface
	fileManager *FileManager
}

func (s *Scheduler) Init() {
	c := gron.New()
	interval := s.config.Persistence.SaveInterval
	statisticInterval := s.config.Statistic.Interval
	c.AddFunc(gron.Every(interval*time.Second), func() {
		go func() {
			err := s.fileManager.SaveToFile(s.config.Persistence.FilePath)

			if err != nil {
				s.logger.Errorf(providers.TypeApp, "Error while persisting data: %s", err)
				return
			}
			s.logger.Infof(providers.TypeApp, "Persisted data to file %s", s.config.Persistence.FilePath)
		}()
	})

	c.AddFunc(gron.Every(statisticInterval*time.Second), func() {
		go func() {
			s.logger.Infof(providers.TypeApp, "Aggregate statistic...")

			s.service.AggregateStats()
			s.logger.Infof(providers.TypeApp, "Statistic aggregated")
		}()

	})

	c.Start()
}

func (s *Scheduler) Restore() error {
	err := s.fileManager.LoadFromFile(s.config.Persistence.FilePath)
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) Persist() error {
	s.logger.Infof(providers.TypeApp, "Persisting statistic to file...")
	err := s.fileManager.SaveToFile(s.config.Persistence.FilePath)
	if err != nil {
		s.logger.Errorf(providers.TypeApp, "Error while persisting data: %s", err)
		return err
	}
	return nil
}

func NewScheduler(config *structures.Config, logger providers.Logger, service services.StatisticServiceInterface, fileManager *FileManager) interfaces.SchedulerInterface {
	return &Scheduler{
		config:      config,
		logger:      logger,
		service:     service,
		fileManager: fileManager,
	}
}
