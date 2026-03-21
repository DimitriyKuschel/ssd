package models

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStatistic() *Statistic {
	return &Statistic{Data: make(map[int]*StatRecord)}
}

func TestStatistic_SetAndGet(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 10, Clicks: 2, Ftr: 1})

	val, ok := s.Get(1)
	require.True(t, ok)
	assert.Equal(t, 10, val.Views)
	assert.Equal(t, 2, val.Clicks)
	assert.Equal(t, 1, val.Ftr)
}

func TestStatistic_GetMissing(t *testing.T) {
	s := newStatistic()
	val, ok := s.Get(999)
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestStatistic_GetReturnsCopy(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 5})

	val, _ := s.Get(1)
	val.Views = 999

	original, _ := s.Get(1)
	assert.Equal(t, 5, original.Views)
}

func TestStatistic_Len(t *testing.T) {
	s := newStatistic()
	assert.Equal(t, 0, s.Len())
	s.Set(1, &StatRecord{})
	s.Set(2, &StatRecord{})
	assert.Equal(t, 2, s.Len())
}

func TestStatistic_PutData(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 1})

	newData := map[int]*StatRecord{
		10: {Views: 100},
	}
	s.PutData(newData)

	assert.Equal(t, 1, s.Len())
	val, ok := s.Get(10)
	require.True(t, ok)
	assert.Equal(t, 100, val.Views)
}

func TestStatistic_GetDataDeepCopy(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 10, Clicks: 5, Ftr: 1})

	copied := s.GetData()
	copied[1].Views = 999
	copied[2] = &StatRecord{Views: 1}

	original, _ := s.Get(1)
	assert.Equal(t, 10, original.Views)
	assert.Equal(t, 1, s.Len())
}

func TestStatistic_IncStats_NewViews(t *testing.T) {
	s := newStatistic()
	input := &InputStats{Views: []string{"1", "2"}}
	s.IncStats(input)

	assert.Equal(t, 2, s.Len())
	v1, _ := s.Get(1)
	assert.Equal(t, 1, v1.Views)
	assert.Equal(t, 0, v1.Clicks)
}

func TestStatistic_IncStats_ExistingViews(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 5, Clicks: 3, Ftr: 0})

	input := &InputStats{Views: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 6, v.Views)
	assert.Equal(t, 3, v.Clicks)
}

func TestStatistic_IncStats_NewClicks(t *testing.T) {
	s := newStatistic()
	input := &InputStats{Clicks: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 0, v.Views)
	assert.Equal(t, 1, v.Clicks)
}

func TestStatistic_IncStats_ExistingClicks(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 10, Clicks: 5})

	input := &InputStats{Clicks: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 10, v.Views)
	assert.Equal(t, 6, v.Clicks)
}

func TestStatistic_IncStats_TrendingHalving(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 512, Clicks: 100, Ftr: 0})

	input := &InputStats{Views: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	// 513 > 512 → ceil(513/2)=257, ceil(100/2)=50, Ftr=1
	assert.Equal(t, 257, v.Views)
	assert.Equal(t, 50, v.Clicks)
	assert.Equal(t, 1, v.Ftr)
}

func TestStatistic_IncStats_InvalidIDs(t *testing.T) {
	s := newStatistic()
	input := &InputStats{
		Views:  []string{"", "abc", "1"},
		Clicks: []string{"", "xyz", "2"},
	}
	s.IncStats(input)

	// Only valid IDs (1, 2) should be added
	assert.Equal(t, 2, s.Len())
	_, ok := s.Get(1)
	assert.True(t, ok)
	_, ok = s.Get(2)
	assert.True(t, ok)
}

func TestStatistic_IncStats_Nil(t *testing.T) {
	s := newStatistic()
	s.IncStats(nil) // should not panic
	assert.Equal(t, 0, s.Len())
}

func TestStatistic_IncStats_ViewsAndClicks(t *testing.T) {
	s := newStatistic()
	input := &InputStats{Views: []string{"1"}, Clicks: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 1, v.Views)
	assert.Equal(t, 1, v.Clicks)
}

func TestStatRecord_BounceRate(t *testing.T) {
	rec := StatRecord{Views: 10, Clicks: 2, Ftr: 0, Hits: 10, Engagements: 6}
	rec.ComputeBounceRate()
	assert.Equal(t, 40, rec.BounceRate) // (10-6)/10*100 = 40

	data, err := json.Marshal(rec)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Equal(t, float64(40), result["br"])
	assert.Equal(t, float64(10), result["h"])
	assert.Equal(t, float64(6), result["e"])
}

func TestStatRecord_BounceRate_ZeroHits(t *testing.T) {
	rec := StatRecord{Views: 10, Clicks: 2}
	rec.ComputeBounceRate()
	assert.Equal(t, 0, rec.BounceRate)
}

func TestStatistic_IncStats_Hits(t *testing.T) {
	s := newStatistic()
	input := &InputStats{Hits: []string{"1"}, Engagements: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 1, v.Hits)
	assert.Equal(t, 1, v.Engagements)
}

func TestStatistic_IncStats_TrendingHalving_IncludesHitsEngagements(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 512, Clicks: 100, Hits: 200, Engagements: 80, Ftr: 0})

	input := &InputStats{Views: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 100, v.Hits)       // (200+1)>>1 = 100
	assert.Equal(t, 40, v.Engagements) // (80+1)>>1 = 40
}

func TestStatRecord_BounceRate_EngagementsExceedHits(t *testing.T) {
	rec := StatRecord{Views: 10, Hits: 5, Engagements: 8}
	rec.ComputeBounceRate()
	assert.Equal(t, 0, rec.BounceRate)
}

func TestStatRecord_BounceRate_100Percent(t *testing.T) {
	rec := StatRecord{Hits: 10, Engagements: 0}
	rec.ComputeBounceRate()
	assert.Equal(t, 100, rec.BounceRate)
}

func TestStatRecord_JSON_Roundtrip(t *testing.T) {
	original := StatRecord{Views: 10, Clicks: 5, Ftr: 2, Hits: 8, Engagements: 3}
	original.ComputeBounceRate()
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored StatRecord
	require.NoError(t, json.Unmarshal(data, &restored))
	assert.Equal(t, original.Views, restored.Views)
	assert.Equal(t, original.Clicks, restored.Clicks)
	assert.Equal(t, original.Ftr, restored.Ftr)
	assert.Equal(t, original.Hits, restored.Hits)
	assert.Equal(t, original.Engagements, restored.Engagements)
	assert.Equal(t, original.BounceRate, restored.BounceRate)
}

func TestStatRecord_JSON_OldFormat(t *testing.T) {
	// Old V4 JSON format (no h/e fields)
	data := []byte(`{"Views":10,"Clicks":5,"Ftr":2}`)
	var rec StatRecord
	require.NoError(t, json.Unmarshal(data, &rec))
	assert.Equal(t, 10, rec.Views)
	assert.Equal(t, 5, rec.Clicks)
	assert.Equal(t, 2, rec.Ftr)
	assert.Equal(t, 0, rec.Hits)
	assert.Equal(t, 0, rec.Engagements)
}

func TestStatistic_GetDataDeepCopy_IncludesHitsEngagements(t *testing.T) {
	s := newStatistic()
	s.Set(1, &StatRecord{Views: 10, Clicks: 5, Ftr: 1, Hits: 8, Engagements: 3})

	copied := s.GetData()
	assert.Equal(t, 8, copied[1].Hits)
	assert.Equal(t, 3, copied[1].Engagements)
	assert.Equal(t, 62, copied[1].BounceRate) // (8-3)*100/8 = 62

	// Mutate copy, verify original unchanged
	copied[1].Hits = 999
	original, _ := s.Get(1)
	assert.Equal(t, 8, original.Hits)
}

func TestStatistic_ConcurrentAccess(t *testing.T) {
	s := newStatistic()
	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			s.IncStats(&InputStats{Views: []string{"1", "2"}, Clicks: []string{"1"}})
		})
	}
	for range 100 {
		wg.Go(func() {
			s.GetData()
		})
	}
	wg.Wait()

	v, ok := s.Get(1)
	assert.True(t, ok)
	assert.Greater(t, v.Views, 0)
}
