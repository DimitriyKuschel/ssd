package models

import "sync"

const maxFingerprints = 100000

type PersonalStats struct {
	mu   sync.RWMutex          `json:"-"`
	Data map[string]*Statistic `json:"data"`
}

func (ps *PersonalStats) IncStats(val *InputStats) {
	if val == nil {
		return
	}

	// Fast path: fingerprint already exists (read lock only)
	ps.mu.RLock()
	stat, ok := ps.Data[val.Fingerprint]
	ps.mu.RUnlock()

	if ok {
		stat.IncStats(val)
		return
	}

	// Slow path: write lock with double-check for new fingerprint
	ps.mu.Lock()
	stat, ok = ps.Data[val.Fingerprint]
	if !ok {
		if len(ps.Data) >= maxFingerprints {
			ps.mu.Unlock()
			return
		}
		stat = &Statistic{
			Data: make(map[int]*StatRecord),
		}
		ps.Data[val.Fingerprint] = stat
	}
	ps.mu.Unlock()

	stat.IncStats(val)
}

func (ps *PersonalStats) Get(key string) (*Statistic, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	val, ok := ps.Data[key]
	return val, ok
}

func (ps *PersonalStats) Set(key string, val *Statistic) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.Data[key] = val
}

func (ps *PersonalStats) Len() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.Data)
}

func (ps *PersonalStats) GetData() map[string]*Statistic {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	copyMap := make(map[string]*Statistic)
	for k, v := range ps.Data {
		copyMap[k] = &Statistic{
			Data: v.GetData(),
		}
	}
	return copyMap
}

func (ps *PersonalStats) PutData(stats map[string]*Statistic) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.Data = stats
}
