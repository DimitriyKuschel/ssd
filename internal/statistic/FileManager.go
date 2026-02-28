package statistic

import (
	json "github.com/goccy/go-json"
	"os"
	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/services"
	"ssd/internal/statistic/interfaces"
	"time"
)

type FileManager struct {
	service    services.StatisticServiceInterface
	compressor interfaces.CompressorInterface
	logger     providers.Logger
}

func NewFileManager(compressor interfaces.CompressorInterface, service services.StatisticServiceInterface, logger providers.Logger) *FileManager {
	return &FileManager{
		compressor: compressor,
		service:    service,
		logger:     logger,
	}
}

func (f *FileManager) SaveToFile(fileName string) error {
	storage := f.service.GetSnapshot()

	jsonData, err := json.Marshal(storage)
	if err != nil {
		return err
	}
	data, err := f.compressor.Compress(jsonData)
	if err != nil {
		return err
	}

	tmpFile := fileName + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		file.Close()
		os.Remove(tmpFile)
		return err
	}

	if err = file.Sync(); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return err
	}

	if err = file.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return os.Rename(tmpFile, fileName)
}

func (f *FileManager) Close() {
	f.compressor.Close()
}

func (f *FileManager) LoadFromFile(fileName string) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	decompressedData, err := f.compressor.Decompress(data)
	if err != nil {
		return err
	}

	// Try V4 format (StorageV4 is a JSON superset of V3 Storage)
	var storageV4 models.StorageV4
	if err := json.Unmarshal(decompressedData, &storageV4); err == nil && storageV4.Channels != nil {
		now := time.Now()
		for ch, cd := range storageV4.Channels {
			if cd.TrendStats == nil {
				cd.TrendStats = make(map[int]*models.StatRecord)
			}
			if cd.PersonalStats == nil {
				cd.PersonalStats = make(map[string]*models.FingerprintPersistence)
			}
			if storageV4.Version < 4 {
				// V3 migration: set LastSeen = now for zero-value timestamps
				migrated := 0
				for _, fp := range cd.PersonalStats {
					if fp.LastSeen.IsZero() {
						fp.LastSeen = now
						migrated++
					}
				}
				if migrated > 0 {
					f.logger.Warnf(providers.TypeApp, "Migrating channel %q from V3 to V4: set lastSeen for %d/%d fingerprints", ch, migrated, len(cd.PersonalStats))
				}
			}
			f.service.PutChannelDataV4(ch, cd.TrendStats, cd.PersonalStats)
		}
		return nil
	}

	// Try old format v2 (trend_stats + personal_stats at top level)
	f.logger.Warnf(providers.TypeApp, "Inconsistent DB found, try to migrate from old data format")
	var oldStorage struct {
		TrendStats    map[int]*models.StatRecord   `json:"trend_stats"`
		PersonalStats map[string]*models.Statistic `json:"personal_stats"`
	}
	if err := json.Unmarshal(decompressedData, &oldStorage); err == nil && oldStorage.TrendStats != nil && oldStorage.PersonalStats != nil {
		f.logger.Warnf(providers.TypeApp, "Migration from v2 format successful")
		f.service.PutChannelData(services.DefaultChannel, oldStorage.TrendStats, oldStorage.PersonalStats)
		return nil
	}

	// Try old format v1 (just map[int]*StatRecord)
	var stats map[int]*models.StatRecord
	if err := json.Unmarshal(decompressedData, &stats); err != nil {
		f.logger.Warnf(providers.TypeApp, "Migration failed")
		return err
	}
	f.logger.Warnf(providers.TypeApp, "Migration from v1 format successful")
	f.service.PutChannelData(services.DefaultChannel, stats, make(map[string]*models.Statistic))

	return nil
}
