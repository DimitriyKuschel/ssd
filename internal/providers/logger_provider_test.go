package providers

import (
	"ssd/internal/structures"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLogTypeByRequestType_POST(t *testing.T) {
	assert.Equal(t, TypeEnum(TypePost), GetLogTypeByRequestType("POST"))
}

func TestGetLogTypeByRequestType_GET(t *testing.T) {
	assert.Equal(t, TypeEnum(TypeGet), GetLogTypeByRequestType("GET"))
}

func TestGetLogTypeByRequestType_Other(t *testing.T) {
	assert.Equal(t, TypeEnum(TypeGet), GetLogTypeByRequestType("PUT"))
	assert.Equal(t, TypeEnum(TypeGet), GetLogTypeByRequestType("DELETE"))
}

func TestNewLogProvider_CreatesLogFiles(t *testing.T) {
	dir := t.TempDir()
	conf := &structures.Config{
		Logger: structures.LoggerConfig{
			Level: "info",
			Mode:  0644,
			Dir:   dir,
		},
		Statistic: structures.StatisticConfig{
			Interval: 10 * time.Second,
		},
	}

	logger, err := NewLogProvider(conf)
	require.NoError(t, err)
	defer logger.Close()

	// Should be able to log without error
	logger.Infof(TypeApp, "test message")
	logger.Debugf(TypeGet, "get message")
	logger.Warnf(TypePost, "post message")
}

func TestNewLogProvider_InvalidDir(t *testing.T) {
	conf := &structures.Config{
		Logger: structures.LoggerConfig{
			Level: "info",
			Mode:  0644,
			Dir:   "/nonexistent/directory/path",
		},
	}

	_, err := NewLogProvider(conf)
	assert.Error(t, err)
}
