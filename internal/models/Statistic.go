package models

import (
	"github.com/spf13/cast"
	"math"
	"sync"
)

type StatRecord struct {
	Views  int
	Clicks int
	Ftr    int
}

type Statistic struct {
	Mutex sync.RWMutex
	Data  map[int]*StatRecord
}

func (sm *Statistic) Get(key int) (*StatRecord, bool) {
	sm.Mutex.RLock()
	defer sm.Mutex.RUnlock()
	val, ok := sm.Data[key]
	return val, ok
}

func (sm *Statistic) Set(key int, val *StatRecord) {
	sm.Mutex.Lock()
	defer sm.Mutex.Unlock()
	sm.Data[key] = val
}

func (sm *Statistic) Len() int {
	sm.Mutex.RLock()
	defer sm.Mutex.RUnlock()
	return len(sm.Data)
}

func (sm *Statistic) PutData(data map[int]*StatRecord) {
	sm.Mutex.Lock()
	defer sm.Mutex.Unlock()
	sm.Data = data
}

func (sm *Statistic) GetData() map[int]*StatRecord {
	sm.Mutex.RLock()
	defer sm.Mutex.RUnlock()

	copyMap := make(map[int]*StatRecord)
	for k, v := range sm.Data {
		copyMap[k] = v
	}
	return copyMap
}

func (sm *Statistic) IncStats(data *InputStats) {
	sm.Mutex.Lock()
	defer sm.Mutex.Unlock()

	if data == nil {
		return
	}

	if data.Views == nil || data.Clicks == nil {
		return
	}

	for _, v := range data.Views {
		if v == "" {
			continue
		}
		key := cast.ToInt(v)
		if _, ok := sm.Data[key]; ok {
			tmp := &StatRecord{
				Views:  sm.Data[key].Views + 1,
				Clicks: sm.Data[key].Clicks,
				Ftr:    sm.Data[key].Ftr,
			}

			if tmp.Views > 512 {
				tmp.Views = int(math.Ceil(float64(tmp.Views / 2)))
				tmp.Clicks = int(math.Ceil(float64(tmp.Clicks / 2)))
				tmp.Ftr += 1
			}

			sm.Data[key] = tmp
		} else {
			sm.Data[key] = &StatRecord{
				Views:  1,
				Clicks: 0,
				Ftr:    0,
			}
		}
	}
	for _, v := range data.Clicks {
		key := cast.ToInt(v)
		if v == "" {
			continue
		}
		if _, ok := sm.Data[key]; ok {
			sm.Data[key] = &StatRecord{
				Views:  sm.Data[key].Views,
				Clicks: sm.Data[key].Clicks + 1,
				Ftr:    sm.Data[key].Ftr,
			}
		} else {
			sm.Data[key] = &StatRecord{
				Views:  0,
				Clicks: 1,
				Ftr:    0,
			}
		}
	}
}
