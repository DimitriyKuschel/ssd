package statistic

import (
	"ssd/internal/providers"
	"ssd/internal/services"
	"ssd/internal/statistic/interfaces"
	"ssd/internal/structures"
	"sync"
	"time"
)

type Scheduler struct {
	config      *structures.Config
	logger      providers.Logger
	service     services.StatisticServiceInterface
	fileManager *FileManager
	metrics     providers.MetricsProviderInterface
	opsMu       sync.Mutex
	stopCh      chan struct{}
}

func (s *Scheduler) Init() {
	s.stopCh = make(chan struct{})

	persistInterval := s.config.Persistence.SaveInterval
	aggregateInterval := s.config.Statistic.Interval

	go func() {
		persistTicker := time.NewTicker(persistInterval)
		aggregateTicker := time.NewTicker(aggregateInterval)
		defer persistTicker.Stop()
		defer aggregateTicker.Stop()

		for {
			select {
			case <-persistTicker.C:
				s.doPersist()
			case <-aggregateTicker.C:
				s.doAggregate()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *Scheduler) doPersist() {
	s.opsMu.Lock()
	defer s.opsMu.Unlock()

	start := time.Now()
	err := s.fileManager.SaveToFile(s.config.Persistence.FilePath)
	s.metrics.ObservePersistenceDuration(time.Since(start))
	if err != nil {
		s.logger.Errorf(providers.TypeApp, "Error while persisting data: %s", err)
		return
	}
	s.logger.Infof(providers.TypeApp, "Persisted data to file %s", s.config.Persistence.FilePath)
}

func (s *Scheduler) doAggregate() {
	s.opsMu.Lock()
	defer s.opsMu.Unlock()

	s.logger.Infof(providers.TypeApp, "Aggregate statistic...")
	s.service.AggregateStats()
	for _, ch := range s.service.GetChannels() {
		s.metrics.SetRecordsTotal(ch, s.service.GetRecordCount(ch))
	}
	s.logger.Infof(providers.TypeApp, "Statistic aggregated")
}

func (s *Scheduler) Stop() {
	if s.stopCh != nil {
		close(s.stopCh)
	}
}

func (s *Scheduler) Close() {
	s.fileManager.Close()
}

func (s *Scheduler) Restore() error {
	err := s.fileManager.LoadFromFile(s.config.Persistence.FilePath)
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) Persist() error {
	s.opsMu.Lock()
	defer s.opsMu.Unlock()

	s.logger.Infof(providers.TypeApp, "Persisting statistic to file...")
	err := s.fileManager.SaveToFile(s.config.Persistence.FilePath)
	if err != nil {
		s.logger.Errorf(providers.TypeApp, "Error while persisting data: %s", err)
		return err
	}
	return nil
}

func NewScheduler(config *structures.Config, logger providers.Logger, service services.StatisticServiceInterface, fileManager *FileManager, metrics providers.MetricsProviderInterface) interfaces.SchedulerInterface {
	return &Scheduler{
		config:      config,
		logger:      logger,
		service:     service,
		fileManager: fileManager,
		metrics:     metrics,
	}
}
