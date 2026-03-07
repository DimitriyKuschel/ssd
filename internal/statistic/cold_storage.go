package statistic

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"

	"ssd/internal/models"
	"ssd/internal/providers"
	"ssd/internal/statistic/interfaces"
)

// ColdEntry represents a single evicted fingerprint in cold storage.
type ColdEntry struct {
	Data      map[int]*models.StatRecord `json:"data"`
	EvictedAt time.Time                  `json:"evicted_at"`
}

// ColdFile represents the on-disk format for a single channel's cold storage.
type ColdFile struct {
	Entries map[string]*ColdEntry `json:"entries"`
}

// ColdStorage handles persistence of evicted fingerprints to disk.
// Implements models.ColdStorageInterface (Has, Evict, Restore).
// Additional methods: Flush, RestoreIndex, Close.
type ColdStorage struct {
	mu         sync.RWMutex
	dir        string
	index      map[string]map[string]struct{}   // channel → set of fingerprint IDs
	pending    map[string]map[string]*ColdEntry // channel → pending entries
	restored   map[string]map[string]struct{}   // channel → fingerprints to lazy-delete
	loaded     map[string]*ColdFile             // channel → cached cold file
	coldTTL    time.Duration
	compressor interfaces.CompressorInterface
	logger     providers.Logger
}

// NewColdStorage creates a new ColdStorage instance.
// dir is the directory for cold storage files.
func NewColdStorage(dir string, coldTTL time.Duration, compressor interfaces.CompressorInterface, logger providers.Logger) *ColdStorage {
	return &ColdStorage{
		dir:        dir,
		index:      make(map[string]map[string]struct{}),
		pending:    make(map[string]map[string]*ColdEntry),
		restored:   make(map[string]map[string]struct{}),
		loaded:     make(map[string]*ColdFile),
		coldTTL:    coldTTL,
		compressor: compressor,
		logger:     logger,
	}
}

// Has checks if a fingerprint exists in cold storage (index or pending).
// Uses RLock for minimal contention on the hot path.
func (cs *ColdStorage) Has(channel, fingerprint string) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if fps, ok := cs.index[channel]; ok {
		_, exists := fps[fingerprint]
		return exists
	}
	return false
}

// Evict adds a fingerprint's data to the pending buffer for later flush.
// No disk I/O is performed.
func (cs *ColdStorage) Evict(channel, fingerprint string, data map[int]*models.StatRecord) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry := &ColdEntry{
		Data:      data,
		EvictedAt: time.Now(),
	}

	if cs.pending[channel] == nil {
		cs.pending[channel] = make(map[string]*ColdEntry)
	}
	cs.pending[channel][fingerprint] = entry

	if cs.index[channel] == nil {
		cs.index[channel] = make(map[string]struct{})
	}
	cs.index[channel][fingerprint] = struct{}{}
}

// Restore retrieves a fingerprint from cold storage (pending or disk).
// Uses lazy load for disk files and lazy delete for restored entries.
func (cs *ColdStorage) Restore(channel, fingerprint string) (map[int]*models.StatRecord, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 1. Check pending buffer first (not yet flushed to disk)
	if entries, ok := cs.pending[channel]; ok {
		if entry, ok := entries[fingerprint]; ok {
			data := entry.Data
			delete(entries, fingerprint)
			if len(entries) == 0 {
				delete(cs.pending, channel)
			}
			delete(cs.index[channel], fingerprint)
			return data, nil
		}
	}

	// 2. Lazy load: read cold file once, cache in loaded
	coldFile := cs.getOrLoadColdFile(channel)
	if coldFile == nil {
		delete(cs.index[channel], fingerprint)
		return nil, nil
	}

	entry, ok := coldFile.Entries[fingerprint]
	if !ok {
		delete(cs.index[channel], fingerprint)
		return nil, nil
	}

	data := entry.Data

	// 3. Lazy delete: mark as restored, actual file rewrite happens in Flush()
	if cs.restored[channel] == nil {
		cs.restored[channel] = make(map[string]struct{})
	}
	cs.restored[channel][fingerprint] = struct{}{}
	delete(cs.index[channel], fingerprint)

	return data, nil
}

// Flush writes all pending entries to disk, applies lazy deletes,
// and cleans up entries older than coldTTL. This is the only method
// that performs disk writes.
func (cs *ColdStorage) Flush() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Collect channels that need updating
	channels := make(map[string]struct{})
	for ch := range cs.pending {
		channels[ch] = struct{}{}
	}
	for ch := range cs.restored {
		channels[ch] = struct{}{}
	}

	for channel := range channels {
		// 1. Get base: loaded cache or load from disk
		coldFile := cs.getOrLoadColdFile(channel)
		if coldFile == nil {
			coldFile = &ColdFile{Entries: make(map[string]*ColdEntry)}
		}

		// 2. Apply lazy deletes (restored fingerprints)
		if restoredFPs, ok := cs.restored[channel]; ok {
			for fp := range restoredFPs {
				delete(coldFile.Entries, fp)
			}
		}

		// 3. Merge pending entries
		if entries, ok := cs.pending[channel]; ok {
			for fp, entry := range entries {
				coldFile.Entries[fp] = entry
			}
		}

		// 4. Cold TTL: remove entries older than coldTTL
		if cs.coldTTL > 0 {
			now := time.Now()
			for fp, entry := range coldFile.Entries {
				if now.Sub(entry.EvictedAt) > cs.coldTTL {
					delete(coldFile.Entries, fp)
					if idx, ok := cs.index[channel]; ok {
						delete(idx, fp)
					}
				}
			}
		}

		// 5. Write atomically or remove empty file
		if len(coldFile.Entries) > 0 {
			if err := cs.writeColdFile(channel, coldFile); err != nil {
				return err
			}
			cs.loaded[channel] = coldFile
		} else {
			os.Remove(cs.coldFilePath(channel))
			delete(cs.loaded, channel)
		}

		// 6. Commit: clear pending/restored only after successful write
		delete(cs.pending, channel)
		delete(cs.restored, channel)
	}
	return nil
}

// RestoreIndex scans the cold storage directory and builds the in-memory
// index of fingerprint IDs. Called once at startup.
func (cs *ColdStorage) RestoreIndex() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := os.MkdirAll(cs.dir, 0755); err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(cs.dir, "*.cold.zst"))
	if err != nil {
		return err
	}

	for _, file := range files {
		channel := cs.extractChannelName(file)
		coldFile := cs.loadColdFileFromDisk(channel)
		if coldFile == nil {
			continue
		}

		cs.index[channel] = make(map[string]struct{}, len(coldFile.Entries))
		for fp := range coldFile.Entries {
			cs.index[channel][fp] = struct{}{}
		}
		// Don't cache loaded data — only index keys
	}
	return nil
}

// Close releases resources held by the compressor.
func (cs *ColdStorage) Close() {
	cs.compressor.Close()
}

// getOrLoadColdFile returns cached cold file or loads it from disk.
// Must be called under cs.mu.Lock().
func (cs *ColdStorage) getOrLoadColdFile(channel string) *ColdFile {
	if cf, ok := cs.loaded[channel]; ok {
		return cf
	}
	cf := cs.loadColdFileFromDisk(channel)
	if cf != nil {
		cs.loaded[channel] = cf
	}
	return cf
}

// loadColdFileFromDisk reads and decompresses a cold file from disk.
func (cs *ColdStorage) loadColdFileFromDisk(channel string) *ColdFile {
	path := cs.coldFilePath(channel)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			cs.logger.Errorf(providers.TypeApp, "Failed to read cold file %s: %s", path, err)
		}
		return nil
	}

	decompressed, err := cs.compressor.Decompress(data)
	if err != nil {
		cs.logger.Errorf(providers.TypeApp, "Failed to decompress cold file %s: %s", path, err)
		return nil
	}

	var cf ColdFile
	if err := json.Unmarshal(decompressed, &cf); err != nil {
		cs.logger.Errorf(providers.TypeApp, "Failed to parse cold file %s: %s", path, err)
		return nil
	}

	if cf.Entries == nil {
		cf.Entries = make(map[string]*ColdEntry)
	}
	return &cf
}

// writeColdFile serializes and atomically writes a cold file to disk.
func (cs *ColdStorage) writeColdFile(channel string, cf *ColdFile) error {
	jsonData, err := json.Marshal(cf)
	if err != nil {
		return err
	}

	compressed, err := cs.compressor.Compress(jsonData)
	if err != nil {
		return err
	}

	path := cs.coldFilePath(channel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, compressed, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, path)
}

// coldFilePath returns the file path for a channel's cold storage file.
func (cs *ColdStorage) coldFilePath(channel string) string {
	return filepath.Join(cs.dir, channel+".cold.zst")
}

// extractChannelName extracts the channel name from a cold file path.
// "default.cold.zst" → "default"
func (cs *ColdStorage) extractChannelName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".cold.zst")
}
