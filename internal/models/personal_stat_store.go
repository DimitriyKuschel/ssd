package models

import (
	"math"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
)

// ColdStorageInterface abstracts cold storage for fingerprint eviction.
// Implemented by statistic.ColdStorage (Phase 3). Nil means no cold storage.
type ColdStorageInterface interface {
	Has(channel, fingerprint string) bool
	Evict(channel, fingerprint string, data map[int]*StatRecord)
	Restore(channel, fingerprint string) (map[int]*StatRecord, error)
}

type PersonalStatStore struct {
	mu              sync.RWMutex
	channel         string
	fingerprints    map[string]*FingerprintRecord
	maxFingerprints int
	maxRecordsPerFP int
	evictionPercent int
	fingerprintTTL  time.Duration
	cold            ColdStorageInterface
}

func NewPersonalStatStore(channel string, maxFingerprints, maxRecordsPerFP, evictionPercent int, fingerprintTTL time.Duration, cold ColdStorageInterface) *PersonalStatStore {
	if maxFingerprints == 0 {
		maxFingerprints = 100000
	}
	if evictionPercent <= 0 {
		evictionPercent = 10
	}
	return &PersonalStatStore{
		channel:         channel,
		fingerprints:    make(map[string]*FingerprintRecord),
		maxFingerprints: maxFingerprints,
		maxRecordsPerFP: maxRecordsPerFP,
		evictionPercent: evictionPercent,
		fingerprintTTL:  fingerprintTTL,
		cold:            cold,
	}
}

func (ps *PersonalStatStore) IncStats(val *InputStats) {
	if val == nil {
		return
	}

	fp := val.Fingerprint

	// Fast path: fingerprint already exists (read lock only)
	ps.mu.RLock()
	rec, ok := ps.fingerprints[fp]
	ps.mu.RUnlock()

	if ok {
		rec.IncStats(val, ps.maxRecordsPerFP, ps.evictionPercent)
		return
	}

	// Slow path: write lock with double-check
	ps.mu.Lock()
	rec, ok = ps.fingerprints[fp]
	if ok {
		ps.mu.Unlock()
		rec.IncStats(val, ps.maxRecordsPerFP, ps.evictionPercent)
		return
	}

	// Try cold storage restore
	rec = ps.tryRestoreFromCold(fp)

	if rec == nil {
		// New fingerprint — check limit
		if ps.maxFingerprints >= 0 && len(ps.fingerprints) >= ps.maxFingerprints {
			ps.mu.Unlock()
			return
		}
		rec = NewFingerprintRecord()
	}
	ps.fingerprints[fp] = rec
	ps.mu.Unlock()

	rec.IncStats(val, ps.maxRecordsPerFP, ps.evictionPercent)
}

// tryRestoreFromCold attempts to restore a fingerprint from cold storage.
// Must be called under ps.mu.Lock().
func (ps *PersonalStatStore) tryRestoreFromCold(fp string) *FingerprintRecord {
	if ps.cold == nil || !ps.cold.Has(ps.channel, fp) {
		return nil
	}
	data, err := ps.cold.Restore(ps.channel, fp)
	if err != nil || data == nil {
		return nil
	}
	return dataToFingerprintRecord(data)
}

// dataToFingerprintRecord converts a map[int]*StatRecord into a FingerprintRecord.
func dataToFingerprintRecord(data map[int]*StatRecord) *FingerprintRecord {
	rec := &FingerprintRecord{
		viewed:   roaring.New(),
		clicked:  roaring.New(),
		counts:   make(map[uint32]StatRecord),
		lastSeen: time.Now(),
	}
	for id, sr := range data {
		if sr == nil || id < 0 || id > math.MaxUint32 {
			continue
		}
		uid := uint32(id)
		if sr.Views > 0 {
			rec.viewed.Add(uid)
		}
		if sr.Clicks > 0 {
			rec.clicked.Add(uid)
		}
		if sr.Views > 1 || sr.Clicks > 1 || sr.Ftr > 0 {
			rec.counts[uid] = *sr
		}
	}
	return rec
}

func (ps *PersonalStatStore) Get(key string) (*Statistic, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	rec, ok := ps.fingerprints[key]
	if !ok {
		return nil, false
	}
	return &Statistic{Data: rec.GetData()}, true
}

func (ps *PersonalStatStore) Len() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.fingerprints)
}

func (ps *PersonalStatStore) GetData() map[string]*Statistic {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make(map[string]*Statistic, len(ps.fingerprints))
	for fp, rec := range ps.fingerprints {
		result[fp] = &Statistic{Data: rec.GetData()}
	}
	return result
}

// PutData loads data from V3 format (map[string]*Statistic) into PersonalStatStore.
// Used for migration and backward compatibility.
func (ps *PersonalStatStore) PutData(stats map[string]*Statistic) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.fingerprints = make(map[string]*FingerprintRecord, len(stats))
	for fp, stat := range stats {
		if stat == nil {
			continue
		}
		ps.fingerprints[fp] = dataToFingerprintRecord(stat.Data)
	}
}

// EvictExpired removes fingerprints that haven't been active for longer than
// fingerprintTTL. If cold storage is available, data is backed up before removal.
func (ps *PersonalStatStore) EvictExpired(now time.Time) {
	if ps.fingerprintTTL <= 0 {
		return
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	for fp, rec := range ps.fingerprints {
		if now.Sub(rec.LastSeen()) > ps.fingerprintTTL {
			if ps.cold != nil {
				ps.cold.Evict(ps.channel, fp, rec.GetData())
			}
			delete(ps.fingerprints, fp)
		}
	}
}
