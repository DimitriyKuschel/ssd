package models

import (
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
)

// FingerprintRecord stores per-fingerprint statistics using sparse counters.
// Bitmaps track which content IDs were viewed/clicked. The counts map only
// holds entries that deviate from the bitmap default (Views>1, Clicks>1, or Ftr>0).
// Thread-safe: all public methods acquire fr.mu internally.
type FingerprintRecord struct {
	mu       sync.RWMutex
	viewed   *roaring.Bitmap
	clicked  *roaring.Bitmap
	counts   map[uint32]StatRecord
	lastSeen time.Time
}

func NewFingerprintRecord() *FingerprintRecord {
	return &FingerprintRecord{
		viewed:   roaring.New(),
		clicked:  roaring.New(),
		counts:   make(map[uint32]StatRecord),
		lastSeen: time.Now(),
	}
}

// IncStats processes views and clicks for this fingerprint.
// maxRecordsPerFP and evictionPercent control per-fingerprint record eviction.
func (fr *FingerprintRecord) IncStats(data *InputStats, maxRecordsPerFP, evictionPercent int) {
	if data == nil {
		return
	}
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.lastSeen = time.Now()
	fr.incViews(data.Views, maxRecordsPerFP, evictionPercent)
	fr.incClicks(data.Clicks)
}

func (fr *FingerprintRecord) incViews(ids []string, maxRecordsPerFP, evictionPercent int) {
	for _, v := range ids {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil || key < 0 || key > math.MaxUint32 {
			continue
		}
		id := uint32(key)
		if fr.viewed.Contains(id) {
			// Repeated view — promote to or update counts
			if rec, ok := fr.counts[id]; ok {
				rec.Views++
				if rec.Views > 512 {
					rec.Views = (rec.Views + 1) >> 1
					rec.Clicks = (rec.Clicks + 1) >> 1
					rec.Ftr++
				}
				fr.counts[id] = rec
			} else {
				// Was bitmap-only (Views: 1), now Views: 2
				newRec := StatRecord{Views: 2}
				if fr.clicked.Contains(id) {
					newRec.Clicks = 1
				}
				fr.counts[id] = newRec
			}
		} else {
			// First view — evict if needed, then add to bitmap
			fr.evictRecords(maxRecordsPerFP, evictionPercent)
			fr.viewed.Add(id)
			// If counts entry already exists (from prior clicks), set Views
			if rec, ok := fr.counts[id]; ok {
				rec.Views = 1
				fr.counts[id] = rec
			}
		}
	}
}

func (fr *FingerprintRecord) incClicks(ids []string) {
	for _, v := range ids {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil || key < 0 || key > math.MaxUint32 {
			continue
		}
		id := uint32(key)
		if fr.clicked.Contains(id) {
			// Repeated click — promote to or update counts
			if rec, ok := fr.counts[id]; ok {
				rec.Clicks++
				fr.counts[id] = rec
			} else {
				// Was bitmap-only (Clicks: 1), now Clicks: 2
				newRec := StatRecord{Clicks: 2}
				if fr.viewed.Contains(id) {
					newRec.Views = 1
				}
				fr.counts[id] = newRec
			}
		} else {
			// First click
			fr.clicked.Add(id)
			// If counts entry already exists (from prior views), set Clicks
			if rec, ok := fr.counts[id]; ok {
				rec.Clicks = 1
				fr.counts[id] = rec
			}
		}
	}
}

// evictRecords removes the least relevant records when the fingerprint
// exceeds maxRecordsPerFP. Eviction is based on viewed bitmap cardinality.
func (fr *FingerprintRecord) evictRecords(maxRecords, evictionPercent int) {
	total := int(fr.viewed.GetCardinality())
	if maxRecords < 0 || total < maxRecords {
		return
	}

	target := int(float64(maxRecords) * float64(evictionPercent) / 100.0)
	if target <= 0 {
		target = 1
	}

	type scored struct {
		id    uint32
		score int
	}
	entries := make([]scored, 0, total)

	it := fr.viewed.Iterator()
	for it.HasNext() {
		id := it.Next()
		score := 1
		if rec, ok := fr.counts[id]; ok {
			score = rec.Views
		}
		entries = append(entries, scored{id: id, score: score})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score < entries[j].score
	})

	for i := 0; i < target && i < len(entries); i++ {
		id := entries[i].id
		fr.viewed.Remove(id)
		fr.clicked.Remove(id)
		delete(fr.counts, id)
	}
}

// GetData reconstructs the full map[int]*StatRecord from bitmaps + sparse counts.
func (fr *FingerprintRecord) GetData() map[int]*StatRecord {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	return fr.buildData()
}

// GetPersistenceData returns data + lastSeen atomically under a single lock.
func (fr *FingerprintRecord) GetPersistenceData() (map[int]*StatRecord, time.Time) {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	return fr.buildData(), fr.lastSeen
}

// buildData reconstructs the full map without locking. Caller must hold fr.mu.
// Uses ToArray() for fast contiguous iteration and a backing slice for bulk allocation.
func (fr *FingerprintRecord) buildData() map[int]*StatRecord {
	viewedArr := fr.viewed.ToArray()
	clickedArr := fr.clicked.ToArray()
	total := len(viewedArr) + len(clickedArr)
	backing := make([]StatRecord, total)
	result := make(map[int]*StatRecord, total)
	idx := 0

	// Pass 1: viewed
	for _, id := range viewedArr {
		if rec, ok := fr.counts[id]; ok {
			backing[idx] = rec
		} else {
			backing[idx] = StatRecord{Views: 1}
		}
		result[int(id)] = &backing[idx]
		idx++
	}

	// Pass 2: clicked — update or create entries
	for _, id := range clickedArr {
		if r, ok := result[int(id)]; ok {
			if _, inCounts := fr.counts[id]; !inCounts {
				r.Clicks = 1
			}
		} else {
			if rec, inCounts := fr.counts[id]; inCounts {
				backing[idx] = rec
			} else {
				backing[idx] = StatRecord{Clicks: 1}
			}
			result[int(id)] = &backing[idx]
			idx++
		}
	}

	return result
}

// LastSeen returns the time of last interaction with this fingerprint.
func (fr *FingerprintRecord) LastSeen() time.Time {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	return fr.lastSeen
}

// ViewedCount returns the number of viewed content IDs.
func (fr *FingerprintRecord) ViewedCount() int {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	return int(fr.viewed.GetCardinality())
}
