package statistic

import (
	"path/filepath"
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
	cold        *ColdStorage
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

	if s.cold != nil {
		if err := s.cold.Flush(); err != nil {
			s.logger.Errorf(providers.TypeApp, "Error flushing cold storage: %s", err)
		}
	}
}

func (s *Scheduler) doAggregate() {
	s.opsMu.Lock()
	defer s.opsMu.Unlock()

	s.logger.Infof(providers.TypeApp, "Aggregate statistic...")
	s.service.AggregateStats()
	s.service.EvictExpiredFingerprints()
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
	if s.cold != nil {
		s.cold.Close()
	}
}

func (s *Scheduler) Restore() error {
	err := s.fileManager.LoadFromFile(s.config.Persistence.FilePath)
	if err != nil {
		return err
	}

	if s.cold != nil {
		if err := s.cold.RestoreIndex(); err != nil {
			s.logger.Errorf(providers.TypeApp, "Error restoring cold storage index: %s", err)
			return err
		}
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

	if s.cold != nil {
		if err := s.cold.Flush(); err != nil {
			s.logger.Errorf(providers.TypeApp, "Error flushing cold storage at shutdown: %s", err)
			return err
		}
	}
	return nil
}

func NewScheduler(config *structures.Config, logger providers.Logger, service services.StatisticServiceInterface, fileManager *FileManager, metrics providers.MetricsProviderInterface) interfaces.SchedulerInterface {
	s := &Scheduler{
		config:      config,
		logger:      logger,
		service:     service,
		fileManager: fileManager,
		metrics:     metrics,
	}

	// Initialize cold storage if fingerprint TTL is configured
	if config.Statistic.FingerprintTTL > 0 {
		coldDir := config.Statistic.ColdStorageDir
		if coldDir == "" {
			coldDir = filepath.Join(filepath.Dir(config.Persistence.FilePath), "fingerprints")
		}

		compressor, err := NewZstdCompressor()
		if err != nil {
			logger.Errorf(providers.TypeApp, "Failed to create cold storage compressor: %s", err)
		} else {
			s.cold = NewColdStorage(coldDir, config.Statistic.ColdTTL, compressor, logger)
			service.SetColdStorage(s.cold)
		}
	}

	return s
}
