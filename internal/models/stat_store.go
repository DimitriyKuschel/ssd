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

	result := make(map[int]*StatRecord, len(s.data))
	for id, rec := range s.data {
		copy := rec
		copy.ComputeBounceRate()
		result[int(id)] = &copy
	}
	return result
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
