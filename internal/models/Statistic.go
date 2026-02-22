package models

import (
	"strconv"
	"sync"
)

type StatRecord struct {
	Views  int
	Clicks int
	Ftr    int
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
	return &StatRecord{Views: val.Views, Clicks: val.Clicks, Ftr: val.Ftr}, true
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
		copyMap[k] = &StatRecord{
			Views:  v.Views,
			Clicks: v.Clicks,
			Ftr:    v.Ftr,
		}
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
}
