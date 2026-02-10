package statistic

import (
	"encoding/json"
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
	fullStats := models.Storage{
		TrendStats:    f.service.GetStatistic(),
		PersonalStats: f.service.GetPersonalStatistic(),
	}

	jsonData, err := json.Marshal(fullStats)
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

	var fullStats models.Storage
	err = json.Unmarshal(decompressedData, &fullStats)
	if err != nil || fullStats.TrendStats == nil || fullStats.PersonalStats == nil {
		f.logger.Warnf(providers.TypeApp, "Inconsistent DB found, try to migrate from old data format")
		var stats map[int]*models.StatRecord
		err = json.Unmarshal(decompressedData, &stats)
		if err != nil {
			f.logger.Warnf(providers.TypeApp, "Migration failed")
			return err
		}
		f.logger.Warnf(providers.TypeApp, "Migration Successful")
		f.service.PutStatistic(stats)
		f.service.PutPersonalStatistic(make(map[string]*models.Statistic))

	} else {
		f.service.PutStatistic(fullStats.TrendStats)
		f.service.PutPersonalStatistic(fullStats.PersonalStats)
	}

	return nil
}
