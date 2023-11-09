package statistic

import (
	"bufio"
	"encoding/json"
	"io"
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

	// Marshal the data to a byte slice
	jsonData, err := json.Marshal(fullStats)
	if err != nil {
		return err
	}
	data, err := f.compressor.Compress(jsonData)

	if err != nil {
		return err
	}

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the compressed data to the binary file
	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func (f *FileManager) LoadFromFile(fileName string) error {
	_, err := os.Stat(fileName)

	if os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	bufferedReader := bufio.NewReader(file)

	var data []byte
	buffer := make([]byte, 1024)
	for {
		n, err := bufferedReader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		data = append(data, buffer[:n]...)
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
