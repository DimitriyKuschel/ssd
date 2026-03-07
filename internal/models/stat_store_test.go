package models

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStatStore() *StatStore {
	return NewStatStore(-1, 10)
}

func TestStatStore_SetAndGet(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 10, Clicks: 2, Ftr: 1})

	val, ok := s.Get(1)
	require.True(t, ok)
	assert.Equal(t, 10, val.Views)
	assert.Equal(t, 2, val.Clicks)
	assert.Equal(t, 1, val.Ftr)
}

func TestStatStore_GetMissing(t *testing.T) {
	s := newStatStore()
	val, ok := s.Get(999)
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestStatStore_GetReturnsCopy(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 5})

	val, _ := s.Get(1)
	val.Views = 999

	original, _ := s.Get(1)
	assert.Equal(t, 5, original.Views)
}

func TestStatStore_Len(t *testing.T) {
	s := newStatStore()
	assert.Equal(t, 0, s.Len())
	s.Set(1, &StatRecord{})
	s.Set(2, &StatRecord{})
	assert.Equal(t, 2, s.Len())
}

func TestStatStore_PutData(t *testing.T) {
	s := newStatStore()
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

func TestStatStore_GetDataDeepCopy(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 10, Clicks: 5, Ftr: 1})

	copied := s.GetData()
	copied[1].Views = 999
	copied[2] = &StatRecord{Views: 1}

	original, _ := s.Get(1)
	assert.Equal(t, 10, original.Views)
	assert.Equal(t, 1, s.Len())
}

func TestStatStore_IncStats_NewViews(t *testing.T) {
	s := newStatStore()
	input := &InputStats{Views: []string{"1", "2"}}
	s.IncStats(input)

	assert.Equal(t, 2, s.Len())
	v1, _ := s.Get(1)
	assert.Equal(t, 1, v1.Views)
	assert.Equal(t, 0, v1.Clicks)
}

func TestStatStore_IncStats_ExistingViews(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 5, Clicks: 3, Ftr: 0})

	input := &InputStats{Views: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 6, v.Views)
	assert.Equal(t, 3, v.Clicks)
}

func TestStatStore_IncStats_NewClicks(t *testing.T) {
	s := newStatStore()
	input := &InputStats{Clicks: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 0, v.Views)
	assert.Equal(t, 1, v.Clicks)
}

func TestStatStore_IncStats_ExistingClicks(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 10, Clicks: 5})

	input := &InputStats{Clicks: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 10, v.Views)
	assert.Equal(t, 6, v.Clicks)
}

func TestStatStore_IncStats_TrendingHalving(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 512, Clicks: 100, Ftr: 0})

	input := &InputStats{Views: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	// 513 > 512 -> ceil(513/2)=257, ceil(100/2)=50 (actually (100+1)>>1=50), Ftr=1
	assert.Equal(t, 257, v.Views)
	assert.Equal(t, 50, v.Clicks) // (100+1)>>1 = 50
	assert.Equal(t, 1, v.Ftr)
}

func TestStatStore_IncStats_InvalidIDs(t *testing.T) {
	s := newStatStore()
	input := &InputStats{
		Views:  []string{"", "abc", "1"},
		Clicks: []string{"", "xyz", "2"},
	}
	s.IncStats(input)

	assert.Equal(t, 2, s.Len())
	_, ok := s.Get(1)
	assert.True(t, ok)
	_, ok = s.Get(2)
	assert.True(t, ok)
}

func TestStatStore_IncStats_NegativeIDs(t *testing.T) {
	s := newStatStore()
	input := &InputStats{
		Views:  []string{"-1", "-100", "5"},
		Clicks: []string{"-2", "10"},
	}
	s.IncStats(input)

	// Only positive IDs should be added
	assert.Equal(t, 2, s.Len())
	_, ok := s.Get(5)
	assert.True(t, ok)
	_, ok = s.Get(10)
	assert.True(t, ok)
}

func TestStatStore_IncStats_OverflowUint32(t *testing.T) {
	s := newStatStore()
	// 5000000000 > math.MaxUint32 (4294967295) — should be skipped
	input := &InputStats{
		Views:  []string{"5000000000", "1"},
		Clicks: []string{"5000000001"},
	}
	s.IncStats(input)

	assert.Equal(t, 1, s.Len())
	_, ok := s.Get(1)
	assert.True(t, ok)
}

func TestStatStore_IncStats_Nil(t *testing.T) {
	s := newStatStore()
	s.IncStats(nil) // should not panic
	assert.Equal(t, 0, s.Len())
}

func TestStatStore_IncStats_ViewsAndClicks(t *testing.T) {
	s := newStatStore()
	input := &InputStats{Views: []string{"1"}, Clicks: []string{"1"}}
	s.IncStats(input)

	v, _ := s.Get(1)
	assert.Equal(t, 1, v.Views)
	assert.Equal(t, 1, v.Clicks)
}

func TestStatStore_Eviction_TriggersAtMax(t *testing.T) {
	s := NewStatStore(10, 50) // max 10 records, evict 50%

	// Fill to max
	for i := 0; i < 10; i++ {
		s.Set(i, &StatRecord{Views: i + 1})
	}
	assert.Equal(t, 10, s.Len())

	// Add one more via IncStats — triggers eviction
	s.IncStats(&InputStats{Views: []string{"100"}})

	// After eviction of 50% (5 records) + adding 1 new = 6
	assert.LessOrEqual(t, s.Len(), 6)
	// The new record should exist
	v, ok := s.Get(100)
	require.True(t, ok)
	assert.Equal(t, 1, v.Views)
}

func TestStatStore_Eviction_RemovesLowestViews(t *testing.T) {
	s := NewStatStore(5, 40) // max 5, evict 40% = 2 records

	// Add records with different Views counts
	s.Set(1, &StatRecord{Views: 1})   // lowest
	s.Set(2, &StatRecord{Views: 2})   // second lowest
	s.Set(3, &StatRecord{Views: 100}) // high
	s.Set(4, &StatRecord{Views: 200}) // higher
	s.Set(5, &StatRecord{Views: 300}) // highest

	// Trigger eviction
	s.IncStats(&InputStats{Views: []string{"99"}})

	// Records with Views 1 and 2 should be evicted
	_, ok := s.Get(1)
	assert.False(t, ok, "record with lowest views should be evicted")
	_, ok = s.Get(2)
	assert.False(t, ok, "record with second lowest views should be evicted")

	// High-value records should remain
	_, ok = s.Get(3)
	assert.True(t, ok)
	_, ok = s.Get(4)
	assert.True(t, ok)
	_, ok = s.Get(5)
	assert.True(t, ok)
}

func TestStatStore_Eviction_UnlimitedWhenMinusOne(t *testing.T) {
	s := NewStatStore(-1, 10) // unlimited

	for i := 0; i < 10000; i++ {
		s.Set(i, &StatRecord{Views: 1})
	}
	assert.Equal(t, 10000, s.Len())
}

func TestStatStore_Eviction_MinTarget(t *testing.T) {
	s := NewStatStore(3, 1) // max 3, evict 1% — should evict at least 1

	s.Set(1, &StatRecord{Views: 1})
	s.Set(2, &StatRecord{Views: 2})
	s.Set(3, &StatRecord{Views: 3})

	// Trigger eviction
	s.IncStats(&InputStats{Views: []string{"99"}})

	// At least 1 record should be evicted
	assert.LessOrEqual(t, s.Len(), 3)
}

func TestStatStore_PutData_NegativeKeysSkipped(t *testing.T) {
	s := newStatStore()
	data := map[int]*StatRecord{
		-1: {Views: 1},
		5:  {Views: 5},
	}
	s.PutData(data)

	assert.Equal(t, 1, s.Len())
	_, ok := s.Get(5)
	assert.True(t, ok)
}

func TestStatStore_PutData_NilValuesSkipped(t *testing.T) {
	s := newStatStore()
	data := map[int]*StatRecord{
		1: nil,
		2: {Views: 2},
	}
	s.PutData(data)

	assert.Equal(t, 1, s.Len())
	_, ok := s.Get(2)
	assert.True(t, ok)
}

func TestStatStore_GetNegativeKey(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 1})

	val, ok := s.Get(-1)
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestStatStore_ConcurrentAccess(t *testing.T) {
	s := newStatStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.IncStats(&InputStats{Views: []string{"1", "2"}, Clicks: []string{"1"}})
		}()
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.GetData()
		}()
	}
	wg.Wait()

	v, ok := s.Get(1)
	assert.True(t, ok)
	assert.Greater(t, v.Views, 0)
}

func TestStatStore_DefaultEvictionPercent(t *testing.T) {
	s := NewStatStore(100, 0) // 0 should default to 10
	assert.Equal(t, 10, s.evictionPercent)

	s = NewStatStore(100, -5) // negative should default to 10
	assert.Equal(t, 10, s.evictionPercent)
}

func TestStatStore_EvictionWithClicks(t *testing.T) {
	s := NewStatStore(3, 50) // max 3, evict 50%

	s.IncStats(&InputStats{Clicks: []string{"1"}})
	s.IncStats(&InputStats{Clicks: []string{"2"}})
	s.IncStats(&InputStats{Clicks: []string{"3"}})

	// All 3 records have Views=0. Trigger eviction via click on new ID
	s.IncStats(&InputStats{Clicks: []string{"99"}})

	// Should not panic, eviction works even with Views=0
	assert.LessOrEqual(t, s.Len(), 3)
	v, ok := s.Get(99)
	require.True(t, ok)
	assert.Equal(t, 1, v.Clicks)
}

func TestStatStore_IncStats_ZeroID(t *testing.T) {
	s := newStatStore()
	input := &InputStats{Views: []string{"0"}}
	s.IncStats(input)

	v, ok := s.Get(0)
	require.True(t, ok)
	assert.Equal(t, 1, v.Views)
}

func TestStatStore_MultipleHalvings(t *testing.T) {
	s := newStatStore()
	s.Set(1, &StatRecord{Views: 512, Clicks: 512, Ftr: 0})

	// First halving
	s.IncStats(&InputStats{Views: []string{"1"}})
	v, _ := s.Get(1)
	assert.Equal(t, 257, v.Views) // (513+1)>>1 = 257
	assert.Equal(t, 1, v.Ftr)

	// Keep incrementing until next halving
	for i := 0; i < 256; i++ {
		s.IncStats(&InputStats{Views: []string{"1"}})
	}

	v, _ = s.Get(1)
	assert.Equal(t, 2, v.Ftr)
}

func BenchmarkStatStore_IncStats(b *testing.B) {
	s := newStatStore()
	input := &InputStats{
		Views:  []string{"1", "2", "3", "4", "5"},
		Clicks: []string{"1", "2"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.IncStats(input)
	}
}

func BenchmarkStatStore_GetData(b *testing.B) {
	s := newStatStore()
	for i := 0; i < 1000; i++ {
		s.Set(i, &StatRecord{Views: i, Clicks: i / 2})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetData()
	}
}

func BenchmarkStatStore_IncStats_WithEviction(b *testing.B) {
	s := NewStatStore(1000, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.IncStats(&InputStats{Views: []string{fmt.Sprintf("%d", i)}})
	}
}
