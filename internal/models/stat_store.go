package models

import (
	"cmp"
	"io"
	"math"
	"slices"
	"strconv"
	"sync"
)

type StatStore struct {
	mu              sync.RWMutex
	data            map[uint32]StatRecord
	maxRecords      int
	evictionPercent int
}

func NewStatStore(maxRecords, evictionPercent int) *StatStore {
	if evictionPercent <= 0 {
		evictionPercent = 10
	}
	return &StatStore{
		data:            make(map[uint32]StatRecord),
		maxRecords:      maxRecords,
		evictionPercent: evictionPercent,
	}
}

func (s *StatStore) IncStats(data *InputStats) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if data == nil {
		return
	}

	for _, v := range data.Views {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil || key < 0 || key > math.MaxUint32 {
			continue
		}
		id := uint32(key)
		if rec, ok := s.data[id]; ok {
			rec.Views++
			if rec.Views > 512 {
				rec.Views = (rec.Views + 1) >> 1
				rec.Clicks = (rec.Clicks + 1) >> 1
				rec.Hits = (rec.Hits + 1) >> 1
				rec.Engagements = (rec.Engagements + 1) >> 1
				rec.Ftr++
			}
			s.data[id] = rec
		} else {
			s.evictIfNeeded()
			s.data[id] = StatRecord{Views: 1}
		}
	}
	for _, v := range data.Clicks {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil || key < 0 || key > math.MaxUint32 {
			continue
		}
		id := uint32(key)
		if rec, ok := s.data[id]; ok {
			rec.Clicks++
			s.data[id] = rec
		} else {
			s.evictIfNeeded()
			s.data[id] = StatRecord{Clicks: 1}
		}
	}
	for _, v := range data.Hits {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil || key < 0 || key > math.MaxUint32 {
			continue
		}
		id := uint32(key)
		if rec, ok := s.data[id]; ok {
			rec.Hits++
			s.data[id] = rec
		} else {
			s.evictIfNeeded()
			s.data[id] = StatRecord{Hits: 1}
		}
	}
	for _, v := range data.Engagements {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil || key < 0 || key > math.MaxUint32 {
			continue
		}
		id := uint32(key)
		if rec, ok := s.data[id]; ok {
			rec.Engagements++
			s.data[id] = rec
		} else {
			s.evictIfNeeded()
			s.data[id] = StatRecord{Engagements: 1}
		}
	}
}

func (s *StatStore) evictIfNeeded() {
	if s.maxRecords < 0 || len(s.data) < s.maxRecords {
		return
	}
	s.evict()
}

func (s *StatStore) evict() {
	target := int(float64(s.maxRecords) * float64(s.evictionPercent) / 100.0)
	if target <= 0 {
		target = 1
	}

	type scored struct {
		id    uint32
		score int
	}
	entries := make([]scored, 0, len(s.data))
	for id, rec := range s.data {
		entries = append(entries, scored{id: id, score: rec.Views})
	}

	slices.SortFunc(entries, func(a, b scored) int {
		return cmp.Compare(a.score, b.score)
	})

	for i := range min(target, len(entries)) {
		delete(s.data, entries[i].id)
	}
}

func (s *StatStore) Get(key int) (*StatRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if key < 0 {
		return nil, false
	}
	val, ok := s.data[uint32(key)]
	if !ok {
		return nil, false
	}
	copy := val
	copy.ComputeBounceRate()
	return &copy, true
}

func (s *StatStore) Set(key int, val *StatRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key < 0 || val == nil {
		return
	}
	s.data[uint32(key)] = *val
}

func (s *StatStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s *StatStore) PutData(data map[int]*StatRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[uint32]StatRecord, len(data))
	for k, v := range data {
		if k < 0 || v == nil {
			continue
		}
		s.data[uint32(k)] = *v
	}
}

func (s *StatStore) GetData() map[int]*StatRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Allocate all records in one backing slice instead of one heap allocation
	// per record (&copy), so a snapshot of N records costs a handful of allocs
	// rather than N. Values point into backing, which is freshly allocated here
	// and never shared, so callers can read freely.
	backing := make([]StatRecord, len(s.data))
	result := make(map[int]*StatRecord, len(s.data))
	idx := 0
	for id, rec := range s.data {
		backing[idx] = rec
		backing[idx].ComputeBounceRate()
		result[int(id)] = &backing[idx]
		idx++
	}
	return result
}

// GetDataPage returns a single page of records sorted by ascending ID, plus the
// total record count. Only the page's records are copied (into one backing
// slice), so a small page over a large channel does not pay to copy every record
// the way GetData() + controller-side slicing would. limit<=0 means "from offset
// to the end".
func (s *StatStore) GetDataPage(limit, offset int) (map[int]*StatRecord, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.data)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return make(map[int]*StatRecord), total
	}

	keys := make([]uint32, 0, total)
	for id := range s.data {
		keys = append(keys, id)
	}
	slices.Sort(keys)

	keys = keys[offset:]
	if limit > 0 && limit < len(keys) {
		keys = keys[:limit]
	}

	backing := make([]StatRecord, len(keys))
	result := make(map[int]*StatRecord, len(keys))
	for i, id := range keys {
		backing[i] = s.data[id]
		backing[i].ComputeBounceRate()
		result[int(id)] = &backing[i]
	}
	return result, total
}

// WriteBinaryTo writes the stat store data in binary format.
func (s *StatStore) WriteBinaryTo(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return writeStatRecords(w, s.data)
}

// ReadBinaryFrom reads stat store data from V6 binary format.
func (s *StatStore) ReadBinaryFrom(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := readStatRecords(r)
	if err != nil {
		return err
	}
	s.data = data
	return nil
}

// ReadBinaryFromV5 reads stat store data from V5 binary format (without hits/engagements).
func (s *StatStore) ReadBinaryFromV5(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := readStatRecordsV5(r)
	if err != nil {
		return err
	}
	s.data = data
	return nil
}
