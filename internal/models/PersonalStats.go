package models

import "sync"

type PersonalStats struct {
	sync.RWMutex `json:"-"`
	Data         map[string]*Statistic `json:"data"`
}

func (ps *PersonalStats) IncStats(val *InputStats) {
	ps.Lock()
	defer ps.Unlock()
	if val == nil {
		return
	}
	if _, ok := ps.Data[val.Fingerprint]; !ok {
		ps.Data[val.Fingerprint] = &Statistic{
			Data: make(map[int]*StatRecord),
		}
	}
	ps.Data[val.Fingerprint].IncStats(val)
}

func (ps *PersonalStats) Get(key string) (*Statistic, bool) {
	ps.RLock()
	defer ps.RUnlock()
	val, ok := ps.Data[key]
	return val, ok
}

func (ps *PersonalStats) Set(key string, val *Statistic) {
	ps.Lock()
	defer ps.Unlock()
	ps.Data[key] = val
}

func (ps *PersonalStats) Len() int {
	ps.RLock()
	defer ps.RUnlock()
	return len(ps.Data)
}

func (ps *PersonalStats) GetData() map[string]*Statistic {
	ps.RLock()
	defer ps.RUnlock()

	copyMap := make(map[string]*Statistic)
	for k, v := range ps.Data {
		copyMap[k] = &Statistic{
			Data: v.GetData(),
		}
	}
	return copyMap
}

func (ps *PersonalStats) PutData(stats map[string]*Statistic) {
	ps.Lock()
	defer ps.Unlock()

	ps.Data = stats
}
