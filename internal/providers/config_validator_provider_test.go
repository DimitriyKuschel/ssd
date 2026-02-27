package providers

import (
	"ssd/internal/structures"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validConfig() *structures.Config {
	return &structures.Config{
		WebServer: structures.Server{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Persistence: structures.Persistence{
			FilePath:     "/tmp/ssd.dat",
			SaveInterval: 30 * time.Second,
		},
		Logger: structures.LoggerConfig{
			Level: "info",
			Mode:  0644,
			Dir:   "/tmp/logs",
		},
		Statistic: structures.StatisticConfig{
			Interval: 10 * time.Second,
		},
	}
}

func TestConfigValidator_ValidConfig(t *testing.T) {
	v := NewCnfValidator(validConfig())
	assert.NoError(t, v.Validate())
}

func TestConfigValidator_EmptyHost(t *testing.T) {
	c := validConfig()
	c.WebServer.Host = ""
	v := NewCnfValidator(c)
	assert.Error(t, v.Validate())
}

func TestConfigValidator_ZeroPort(t *testing.T) {
	c := validConfig()
	c.WebServer.Port = 0
	v := NewCnfValidator(c)
	assert.Error(t, v.Validate())
}

func TestConfigValidator_EmptyLogLevel(t *testing.T) {
	c := validConfig()
	c.Logger.Level = ""
	v := NewCnfValidator(c)
	assert.Error(t, v.Validate())
}

func TestConfigValidator_InvalidLogLevel(t *testing.T) {
	c := validConfig()
	c.Logger.Level = "verbose"
	v := NewCnfValidator(c)
	assert.Error(t, v.Validate())
}
