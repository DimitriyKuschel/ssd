package structures

import "time"

type Server struct {
	Host string `yaml:"host" validate:"required"`
	Port int    `yaml:"port" validate:"required|uint|min:1"`
}

type Persistence struct {
	FilePath     string        `yaml:"filePath" validate:"required|unixPath"`
	SaveInterval time.Duration `yaml:"saveInterval" validate:"required|min:1"`
}

type LoggerConfig struct {
	Level string `yaml:"level" validate:"required|in:trace,debug,info,warn,error,fatal,panic"`
	Mode  uint32 `yaml:"mode" validate:"required|uint"`
	Dir   string `yaml:"dir" validate:"required|unixPath"`
}

type StatisticConfig struct {
	Interval        time.Duration `yaml:"interval" validate:"required|min:1"`
	MaxChannels     int           `yaml:"maxChannels"`
	MaxRecords      int           `yaml:"maxRecords"`
	EvictionPercent int           `yaml:"evictionPercent"`
	MaxRecordsPerFP int           `yaml:"maxRecordsPerFingerprint"`
	FingerprintTTL  time.Duration `yaml:"fingerprintTTL"`
	ColdStorageDir  string        `yaml:"coldStorageDir"`
	ColdTTL         time.Duration `yaml:"coldTTL"`
}

type CacheConfig struct {
	Enabled bool `yaml:"enabled"`
	Size    int  `yaml:"size"`
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
}

type Config struct {
	AppName     string
	Debug       bool
	Path        string
	Statistic   StatisticConfig `yaml:"statistic"`
	WebServer   Server          `yaml:"webServer"`
	Persistence Persistence     `yaml:"persistence"`
	Logger      LoggerConfig    `yaml:"logger"`
	Cache       CacheConfig     `yaml:"cache"`
	Metrics     MetricsConfig   `yaml:"metrics"`
}
