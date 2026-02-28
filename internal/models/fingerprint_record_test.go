package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFR() *FingerprintRecord {
	return NewFingerprintRecord()
}

func TestFR_NewHasEmptyBitmaps(t *testing.T) {
	fr := newFR()
	assert.Equal(t, 0, fr.ViewedCount())
	assert.Empty(t, fr.counts)
	assert.False(t, fr.lastSeen.IsZero())
}

func TestFR_SingleView(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)

	data := fr.GetData()
	require.Len(t, data, 1)
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 0, data[1].Clicks)
	// Should be bitmap-only, no counts entry
	assert.Empty(t, fr.counts)
}

func TestFR_SingleClick(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Clicks: []string{"5"}}, -1, 10)

	data := fr.GetData()
	require.Len(t, data, 1)
	assert.Equal(t, 0, data[5].Views)
	assert.Equal(t, 1, data[5].Clicks)
	assert.Empty(t, fr.counts)
}

func TestFR_ViewAndClick(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Views: []string{"1"}, Clicks: []string{"1"}}, -1, 10)

	data := fr.GetData()
	require.Len(t, data, 1)
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
	// Both are 1, no counts needed
	assert.Empty(t, fr.counts)
}

func TestFR_RepeatedView_PromoToCounts(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 2, data[1].Views)
	// Should now be in counts
	assert.Len(t, fr.counts, 1)
}

func TestFR_RepeatedClick_PromoToCounts(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Clicks: []string{"3"}}, -1, 10)
	fr.IncStats(&InputStats{Clicks: []string{"3"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 2, data[3].Clicks)
	assert.Len(t, fr.counts, 1)
}

func TestFR_ClickTwiceThenView(t *testing.T) {
	fr := newFR()
	// Click twice -> counts[1] = {Clicks: 2}
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)
	// View once -> should update counts[1].Views = 1
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 2, data[1].Clicks)
}

func TestFR_ViewTwiceThenClick(t *testing.T) {
	fr := newFR()
	// View twice -> counts[1] = {Views: 2}
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)
	// Click once -> should update counts[1].Clicks = 1
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 2, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
}

func TestFR_ClickOnceThenViewTwice(t *testing.T) {
	fr := newFR()
	// Click once -> clicked bitmap only
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)
	// View once -> viewed bitmap, no counts (both are 1)
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)
	// View again -> promote to counts with Clicks: 1 preserved
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 2, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
}

func TestFR_ViewOnceThenClickTwice(t *testing.T) {
	fr := newFR()
	// View once -> viewed bitmap only
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)
	// Click once -> clicked bitmap, no counts
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)
	// Click again -> promote to counts with Views: 1 preserved
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 2, data[1].Clicks)
}

func TestFR_TrendingHalving(t *testing.T) {
	fr := newFR()
	// Set up: 512 views via counts
	fr.viewed.Add(1)
	fr.counts[1] = StatRecord{Views: 512, Clicks: 100}

	// One more view triggers halving
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 257, data[1].Views) // (513+1)>>1
	assert.Equal(t, 50, data[1].Clicks) // (100+1)>>1
	assert.Equal(t, 1, data[1].Ftr)
}

func TestFR_SparseCounts_MostSingleView(t *testing.T) {
	fr := newFR()
	// 100 IDs with single view
	for i := 0; i < 100; i++ {
		fr.IncStats(&InputStats{Views: []string{itoa(i)}}, -1, 10)
	}
	// Only 3 with repeated views
	fr.IncStats(&InputStats{Views: []string{"0", "1", "2"}}, -1, 10)

	assert.Equal(t, 100, fr.ViewedCount())
	assert.Len(t, fr.counts, 3) // sparse: only 3 out of 100

	data := fr.GetData()
	assert.Len(t, data, 100)
	assert.Equal(t, 2, data[0].Views)
	assert.Equal(t, 1, data[50].Views) // single view — from bitmap
}

func TestFR_GetData_ClickOnlyEntries(t *testing.T) {
	fr := newFR()
	// ID 1: click only, no view
	fr.IncStats(&InputStats{Clicks: []string{"1"}}, -1, 10)
	// ID 2: view only
	fr.IncStats(&InputStats{Views: []string{"2"}}, -1, 10)

	data := fr.GetData()
	assert.Equal(t, 0, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
	assert.Equal(t, 1, data[2].Views)
	assert.Equal(t, 0, data[2].Clicks)
}

func TestFR_InvalidIDs(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{
		Views:  []string{"", "abc", "-1", "1"},
		Clicks: []string{"", "xyz", "-2", "2"},
	}, -1, 10)

	data := fr.GetData()
	assert.Len(t, data, 2)
}

func TestFR_Nil(t *testing.T) {
	fr := newFR()
	fr.IncStats(nil, -1, 10)
	assert.Equal(t, 0, fr.ViewedCount())
}

func TestFR_EvictRecords(t *testing.T) {
	fr := newFR()
	// Add 10 views with different counts
	for i := 0; i < 10; i++ {
		fr.viewed.Add(uint32(i))
		if i < 5 {
			// IDs 0-4 have low views
			fr.counts[uint32(i)] = StatRecord{Views: i + 1}
		}
		// IDs 5-9 are bitmap-only (Views: 1) — even lower score
	}

	// Evict 50% = 5 records (the 5 with lowest scores)
	fr.evictRecords(10, 50)

	assert.Equal(t, 5, fr.ViewedCount())
	// IDs 5-9 had score 1, IDs 0-4 had scores 1-5
	// After eviction: 5 lowest are removed
}

func TestFR_EvictRecords_UnlimitedSkips(t *testing.T) {
	fr := newFR()
	for i := 0; i < 100; i++ {
		fr.viewed.Add(uint32(i))
	}
	fr.evictRecords(-1, 10) // unlimited — should not evict
	assert.Equal(t, 100, fr.ViewedCount())
}

func TestFR_EvictRecords_RemovesFromAllMaps(t *testing.T) {
	fr := newFR()
	fr.viewed.Add(1)
	fr.clicked.Add(1)
	fr.counts[1] = StatRecord{Views: 1, Clicks: 1}

	fr.evictRecords(1, 100) // max 1, evict 100% = 1 record

	assert.Equal(t, 0, fr.ViewedCount())
	assert.False(t, fr.clicked.Contains(1))
	assert.Empty(t, fr.counts)
}

func TestFR_EvictOnNewView(t *testing.T) {
	fr := newFR()
	// Fill to max
	for i := 0; i < 5; i++ {
		fr.IncStats(&InputStats{Views: []string{itoa(i)}}, 5, 40)
	}
	assert.Equal(t, 5, fr.ViewedCount())

	// Add one more — triggers eviction of 40% = 2
	fr.IncStats(&InputStats{Views: []string{"99"}}, 5, 40)

	assert.LessOrEqual(t, fr.ViewedCount(), 4) // 5 - 2 + 1 = 4
	data := fr.GetData()
	assert.Contains(t, data, 99) // new record exists
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func TestFR_GetData_EmptyRecord(t *testing.T) {
	fr := newFR()
	data := fr.GetData()
	assert.NotNil(t, data)
	assert.Empty(t, data)
}

func TestFR_GetPersistenceData_ReturnsDataAndLastSeen(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Views: []string{"1", "2"}, Clicks: []string{"1"}}, -1, 10)

	data, lastSeen := fr.GetPersistenceData()

	require.Len(t, data, 2)
	assert.Equal(t, 1, data[1].Views)
	assert.Equal(t, 1, data[1].Clicks)
	assert.Equal(t, 1, data[2].Views)
	assert.False(t, lastSeen.IsZero())
	assert.WithinDuration(t, time.Now(), lastSeen, 1*time.Second)
}

func TestFR_GetPersistenceData_MatchesGetData(t *testing.T) {
	fr := newFR()
	fr.IncStats(&InputStats{Views: []string{"1"}}, -1, 10)
	fr.IncStats(&InputStats{Views: []string{"1"}, Clicks: []string{"1", "3"}}, -1, 10)

	dataOnly := fr.GetData()
	dataPersist, _ := fr.GetPersistenceData()

	require.Equal(t, len(dataOnly), len(dataPersist))
	for id, rec := range dataOnly {
		pr, ok := dataPersist[id]
		require.True(t, ok)
		assert.Equal(t, rec.Views, pr.Views)
		assert.Equal(t, rec.Clicks, pr.Clicks)
		assert.Equal(t, rec.Ftr, pr.Ftr)
	}
}

func TestFR_GetPersistenceData_EmptyRecord(t *testing.T) {
	fr := newFR()
	data, lastSeen := fr.GetPersistenceData()
	assert.NotNil(t, data)
	assert.Empty(t, data)
	assert.False(t, lastSeen.IsZero())
}
