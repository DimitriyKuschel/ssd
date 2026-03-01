package providers

import (
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
	"ssd/internal/structures"
	"strings"
)

func NewConfigProvider(flags *structures.CliFlags) (*structures.Config, error) {
	var conf structures.Config

	filename := filepath.Base(flags.ConfigPath)
	viper.AddConfigPath(filepath.Dir(flags.ConfigPath))
	viper.SetConfigName(strings.TrimSuffix(filename, filepath.Ext(filename)))
	viper.SetConfigType("yaml")

	viper.SetDefault("statistic.maxChannels", 1000)
	viper.SetDefault("statistic.maxRecords", -1)
	viper.SetDefault("statistic.evictionPercent", 10)
	viper.SetDefault("statistic.maxRecordsPerFingerprint", -1)
	viper.SetDefault("statistic.fingerprintTTL", 0)
	viper.SetDefault("statistic.coldStorageDir", "")
	viper.SetDefault("statistic.coldTTL", 0)

	viper.BindEnv("logger.level", "SSD_LOG_LEVEL")
	viper.BindEnv("statistic.interval", "SSD_AGGREGATION_INTERVAL")
	viper.BindEnv("statistic.maxChannels", "SSD_MAX_CHANNELS")
	viper.BindEnv("statistic.maxRecords", "SSD_MAX_RECORDS")
	viper.BindEnv("statistic.evictionPercent", "SSD_EVICTION_PERCENT")
	viper.BindEnv("statistic.maxRecordsPerFingerprint", "SSD_MAX_RECORDS_PER_FP")
	viper.BindEnv("statistic.fingerprintTTL", "SSD_FINGERPRINT_TTL")
	viper.BindEnv("statistic.coldStorageDir", "SSD_COLD_STORAGE_DIR")
	viper.BindEnv("statistic.coldTTL", "SSD_COLD_TTL")
	viper.BindEnv("persistence.saveInterval", "SSD_SAVE_INTERVAL")
	viper.BindEnv("cache.enabled", "SSD_CACHE_ENABLED")
	viper.BindEnv("cache.size", "SSD_CACHE_SIZE")
	viper.BindEnv("metrics.enabled", "SSD_METRICS_ENABLED")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&conf)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	cnfValidator := NewCnfValidator(&conf)
	err = cnfValidator.Validate()
	if err != nil {
		return nil, err
	}

	conf.AppName = "SimpleStatisticDaemon"
	conf.Path = flags.ConfigPath
	conf.Debug = flags.DebugMode

	return &conf, nil
}
