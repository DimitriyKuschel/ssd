package models

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPersonalStats() *PersonalStats {
	return &PersonalStats{Data: make(map[string]*Statistic)}
}

func TestPersonalStats_SetAndGet(t *testing.T) {
	ps := newPersonalStats()
	s := &Statistic{Data: map[int]*StatRecord{1: {Views: 5}}}
	ps.Set("fp1", s)

	val, ok := ps.Get("fp1")
	require.True(t, ok)
	assert.Equal(t, 1, val.Len())
}

func TestPersonalStats_GetMissing(t *testing.T) {
	ps := newPersonalStats()
	val, ok := ps.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestPersonalStats_Len(t *testing.T) {
	ps := newPersonalStats()
	assert.Equal(t, 0, ps.Len())
	ps.Set("fp1", &Statistic{Data: make(map[int]*StatRecord)})
	ps.Set("fp2", &Statistic{Data: make(map[int]*StatRecord)})
	assert.Equal(t, 2, ps.Len())
}

func TestPersonalStats_PutData(t *testing.T) {
	ps := newPersonalStats()
	ps.Set("old", &Statistic{Data: make(map[int]*StatRecord)})

	newData := map[string]*Statistic{
		"new": {Data: map[int]*StatRecord{1: {Views: 10}}},
	}
	ps.PutData(newData)

	assert.Equal(t, 1, ps.Len())
	val, ok := ps.Get("new")
	require.True(t, ok)
	assert.Equal(t, 1, val.Len())
}

func TestPersonalStats_GetDataDeepCopy(t *testing.T) {
	ps := newPersonalStats()
	ps.Set("fp1", &Statistic{Data: map[int]*StatRecord{1: {Views: 10}}})

	copied := ps.GetData()
	copied["fp1"].Data[1].Views = 999

	original, _ := ps.Get("fp1")
	rec, _ := original.Get(1)
	assert.Equal(t, 10, rec.Views)
}

func TestPersonalStats_IncStats_NewFingerprint(t *testing.T) {
	ps := newPersonalStats()
	input := &InputStats{Fingerprint: "fp1", Views: []string{"1", "2"}}
	ps.IncStats(input)

	assert.Equal(t, 1, ps.Len())
	val, ok := ps.Get("fp1")
	require.True(t, ok)
	assert.Equal(t, 2, val.Len())
}

func TestPersonalStats_IncStats_ExistingFingerprint(t *testing.T) {
	ps := newPersonalStats()
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1"}})
	ps.IncStats(&InputStats{Fingerprint: "fp1", Views: []string{"1", "2"}})

	val, _ := ps.Get("fp1")
	assert.Equal(t, 2, val.Len())
	rec, _ := val.Get(1)
	assert.Equal(t, 2, rec.Views)
}

func TestPersonalStats_IncStats_Nil(t *testing.T) {
	ps := newPersonalStats()
	ps.IncStats(nil) // should not panic
	assert.Equal(t, 0, ps.Len())
}

func TestPersonalStats_MaxFingerprints(t *testing.T) {
	ps := newPersonalStats()

	// Fill to max
	for i := 0; i < maxFingerprints; i++ {
		ps.Data[fmt.Sprintf("fp%d", i)] = &Statistic{Data: make(map[int]*StatRecord)}
	}
	assert.Equal(t, maxFingerprints, ps.Len())

	// Try to add one more — should be rejected
	ps.IncStats(&InputStats{Fingerprint: "overflow", Views: []string{"1"}})
	_, ok := ps.Get("overflow")
	assert.False(t, ok)
	assert.Equal(t, maxFingerprints, ps.Len())
}

func TestPersonalStats_MaxFingerprints_ExistingStillWorks(t *testing.T) {
	ps := newPersonalStats()
	for i := 0; i < maxFingerprints; i++ {
		ps.Data[fmt.Sprintf("fp%d", i)] = &Statistic{Data: make(map[int]*StatRecord)}
	}

	// Existing fingerprint should still get updates
	ps.IncStats(&InputStats{Fingerprint: "fp0", Views: []string{"1"}})
	val, ok := ps.Get("fp0")
	require.True(t, ok)
	assert.Equal(t, 1, val.Len())
}

func TestPersonalStats_ConcurrentAccess(t *testing.T) {
	ps := newPersonalStats()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ps.IncStats(&InputStats{
				Fingerprint: fmt.Sprintf("fp%d", i%10),
				Views:       []string{"1"},
			})
		}(i)
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ps.GetData()
		}()
	}
	wg.Wait()

	assert.Greater(t, ps.Len(), 0)
	assert.LessOrEqual(t, ps.Len(), 10)
}
