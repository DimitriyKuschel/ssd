package statistic

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"ssd/internal/models"
	"ssd/internal/services"
	"ssd/internal/statistic/interfaces"
)

type FileManager struct {
	service    services.StatisticServiceInterface
	compressor interfaces.CompressorInterface
}

func NewFileManager(compressor interfaces.CompressorInterface, service services.StatisticServiceInterface) *FileManager {
	return &FileManager{
		compressor: compressor,
		service:    service,
	}
}
func (f *FileManager) SaveToFile(fileName string) error {

	// Marshal the data to a byte slice
	jsonData, err := json.Marshal(f.service.GetStatistic())
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

	var stats map[int]*models.StatRecord
	err = json.Unmarshal(decompressedData, &stats)
	if err != nil {
		return err
	}

	f.service.PutStatistic(stats)

	return nil
}
