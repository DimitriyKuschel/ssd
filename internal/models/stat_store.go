package models

import (
	"io"
	"math"
	"sort"
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

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score < entries[j].score
	})

	for i := 0; i < target && i < len(entries); i++ {
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

// ReadBinaryFrom reads stat store data from binary format.
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
