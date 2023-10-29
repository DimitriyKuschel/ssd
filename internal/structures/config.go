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
	Interval time.Duration `yaml:"interval" validate:"required|min:1"`
}

type Config struct {
	AppName     string
	Debug       bool
	Path        string
	Statistic   StatisticConfig `yaml:"statistic"`
	WebServer   Server          `yaml:"webServer"`
	Persistence Persistence     `yaml:"persistence"`
	Logger      LoggerConfig    `yaml:"logger"`
}
