package testutil

import (
	"ssd/internal/models"
	"ssd/internal/providers"
	"sync"
)

// MockLogger implements providers.Logger and records calls.
type MockLogger struct {
	mu   sync.Mutex
	Logs []LogEntry
}

type LogEntry struct {
	Level  string
	Type   providers.TypeEnum
	Format string
	Args   []interface{}
}

func (m *MockLogger) record(level string, t providers.TypeEnum, format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logs = append(m.Logs, LogEntry{Level: level, Type: t, Format: format, Args: args})
}

func (m *MockLogger) Errorf(t providers.TypeEnum, format string, args ...interface{}) {
	m.record("error", t, format, args...)
}
func (m *MockLogger) Warnf(t providers.TypeEnum, format string, args ...interface{}) {
	m.record("warn", t, format, args...)
}
func (m *MockLogger) Debugf(t providers.TypeEnum, format string, args ...interface{}) {
	m.record("debug", t, format, args...)
}
func (m *MockLogger) Infof(t providers.TypeEnum, format string, args ...interface{}) {
	m.record("info", t, format, args...)
}
func (m *MockLogger) Fatalf(t providers.TypeEnum, format string, args ...interface{}) {
	m.record("fatal", t, format, args...)
}
func (m *MockLogger) Close() {}

// MockStatisticService implements services.StatisticServiceInterface.
type MockStatisticService struct {
	mu              sync.Mutex
	AddStatsCalls   []*models.InputStats
	AggregateCalls  int
	StatisticData   map[string]map[int]*models.StatRecord
	PersonalData    map[string]map[string]*models.Statistic
	FingerprintData map[string]map[int]*models.StatRecord // key: "channel:fp"
	ChannelsList    []string
	PutCalls        []PutChannelCall
}

type PutChannelCall struct {
	Channel  string
	Trend    map[int]*models.StatRecord
	Personal map[string]*models.Statistic
}

func (m *MockStatisticService) AddStats(data *models.InputStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AddStatsCalls = append(m.AddStatsCalls, data)
}

func (m *MockStatisticService) AggregateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AggregateCalls++
}

func (m *MockStatisticService) GetStatistic(channel string) map[int]*models.StatRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.StatisticData != nil {
		return m.StatisticData[channel]
	}
	return nil
}

func (m *MockStatisticService) GetPersonalStatistic(channel string) map[string]*models.Statistic {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.PersonalData != nil {
		return m.PersonalData[channel]
	}
	return nil
}

func (m *MockStatisticService) GetByFingerprint(channel, fp string) map[int]*models.StatRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FingerprintData != nil {
		return m.FingerprintData[channel+":"+fp]
	}
	return nil
}

func (m *MockStatisticService) PutChannelData(channel string, trend map[int]*models.StatRecord, personal map[string]*models.Statistic) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PutCalls = append(m.PutCalls, PutChannelCall{Channel: channel, Trend: trend, Personal: personal})
}

func (m *MockStatisticService) GetChannels() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ChannelsList
}

func (m *MockStatisticService) GetSnapshot() *models.Storage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &models.Storage{
		Channels: make(map[string]*models.ChannelData),
	}
}

// MockCache implements providers.CacheProviderInterface.
type MockCache struct {
	mu   sync.Mutex
	Data map[string][]byte
}

func NewMockCache() *MockCache {
	return &MockCache{Data: make(map[string][]byte)}
}

func (m *MockCache) Get(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.Data[key]
	return val, ok
}

func (m *MockCache) Set(key string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Data[key] = value
}

// MockCompressor implements interfaces.CompressorInterface with injectable behavior.
type MockCompressor struct {
	CompressFn   func([]byte) ([]byte, error)
	DecompressFn func([]byte) ([]byte, error)
}

func (m *MockCompressor) Compress(val []byte) ([]byte, error) {
	if m.CompressFn != nil {
		return m.CompressFn(val)
	}
	// Default: return as-is (identity)
	out := make([]byte, len(val))
	copy(out, val)
	return out, nil
}

func (m *MockCompressor) Decompress(val []byte) ([]byte, error) {
	if m.DecompressFn != nil {
		return m.DecompressFn(val)
	}
	out := make([]byte, len(val))
	copy(out, val)
	return out, nil
}
