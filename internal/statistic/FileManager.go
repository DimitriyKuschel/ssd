package statistic

import (
	json "github.com/goccy/go-json"
	"os"
	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/services"
	"ssd/internal/statistic/interfaces"
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

	// Try new format (with channels)
	var storage models.Storage
	if err := json.Unmarshal(decompressedData, &storage); err == nil && storage.Channels != nil {
		for ch, cd := range storage.Channels {
			if cd.TrendStats == nil {
				cd.TrendStats = make(map[int]*models.StatRecord)
			}
			if cd.PersonalStats == nil {
				cd.PersonalStats = make(map[string]*models.Statistic)
			}
			f.service.PutChannelData(ch, cd.TrendStats, cd.PersonalStats)
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
