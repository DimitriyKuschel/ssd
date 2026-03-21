package models

import (
	"strconv"
	"sync"
)

type StatRecord struct {
	Views       int
	Clicks      int
	Ftr         int
	Hits        int `json:"h"`
	Engagements int `json:"e"`
	BounceRate  int `json:"br"`
}

// ComputeBounceRate sets BounceRate from Hits and Engagements.
// Called during GetData() on copies — never persisted.
func (sr *StatRecord) ComputeBounceRate() {
	if sr.Hits > 0 {
		sr.BounceRate = max((sr.Hits-sr.Engagements)*100/sr.Hits, 0)
	}
}

type Statistic struct {
	mutex sync.RWMutex        `json:"-"`
	Data  map[int]*StatRecord `json:"data"`
}

func (sm *Statistic) Get(key int) (*StatRecord, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	val, ok := sm.Data[key]
	if !ok {
		return nil, false
	}
	rec := &StatRecord{Views: val.Views, Clicks: val.Clicks, Ftr: val.Ftr, Hits: val.Hits, Engagements: val.Engagements}
	rec.ComputeBounceRate()
	return rec, true
}

func (sm *Statistic) Set(key int, val *StatRecord) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.Data[key] = val
}

func (sm *Statistic) Len() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.Data)
}

func (sm *Statistic) PutData(data map[int]*StatRecord) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.Data = data
}

func (sm *Statistic) GetData() map[int]*StatRecord {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	copyMap := make(map[int]*StatRecord, len(sm.Data))
	for k, v := range sm.Data {
		rec := &StatRecord{
			Views:       v.Views,
			Clicks:      v.Clicks,
			Ftr:         v.Ftr,
			Hits:        v.Hits,
			Engagements: v.Engagements,
		}
		rec.ComputeBounceRate()
		copyMap[k] = rec
	}
	return copyMap
}

func (sm *Statistic) IncStats(data *InputStats) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if data == nil {
		return
	}

	for _, v := range data.Views {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if existing, ok := sm.Data[key]; ok {
			existing.Views++
			if existing.Views > 512 {
				existing.Views = (existing.Views + 1) >> 1
				existing.Clicks = (existing.Clicks + 1) >> 1
				existing.Hits = (existing.Hits + 1) >> 1
				existing.Engagements = (existing.Engagements + 1) >> 1
				existing.Ftr++
			}
		} else {
			sm.Data[key] = &StatRecord{Views: 1}
		}
	}
	for _, v := range data.Clicks {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if existing, ok := sm.Data[key]; ok {
			existing.Clicks++
		} else {
			sm.Data[key] = &StatRecord{Clicks: 1}
		}
	}
	for _, v := range data.Hits {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if existing, ok := sm.Data[key]; ok {
			existing.Hits++
		} else {
			sm.Data[key] = &StatRecord{Hits: 1}
		}
	}
	for _, v := range data.Engagements {
		if v == "" {
			continue
		}
		key, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if existing, ok := sm.Data[key]; ok {
			existing.Engagements++
		} else {
			sm.Data[key] = &StatRecord{Engagements: 1}
		}
	}
}
