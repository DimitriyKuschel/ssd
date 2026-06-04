package models

import (
	"fmt"
	"slices"
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
)

// BenchmarkBuildData measures buildData() with various record counts.
func BenchmarkBuildData(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			fr := &FingerprintRecord{
				viewed:  roaring.New(),
				clicked: roaring.New(),
				counts:  make(map[uint32]StatRecord),
			}
			for i := uint32(0); i < uint32(n); i++ {
				fr.viewed.Add(i)
				if i%3 == 0 {
					fr.clicked.Add(i)
				}
				if i%5 == 0 {
					fr.counts[i] = StatRecord{Views: 10, Clicks: 3, Ftr: 1}
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				fr.buildData()
			}
		})
	}
}

// BenchmarkStatStoreGetData measures StatStore.GetData() with various record counts.
func BenchmarkStatStoreGetData(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			s := NewStatStore(-1, 10)
			for i := range n {
				s.data[uint32(i)] = StatRecord{Views: 10, Clicks: 3, Ftr: 1, Hits: 4, Engagements: 2}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				s.GetData()
			}
		})
	}
}

// BenchmarkStatStoreGetData_FullVsPage contrasts the two end-to-end ways of
// serving a small paginated read over a large channel:
//   - "old": copy the whole channel (GetData) then sort+slice to the page, as the
//     controller used to do via paginateIntMap.
//   - "new": GetDataPage, which sorts keys but copies only the page.
func BenchmarkStatStoreGetData_FullVsPage(b *testing.B) {
	const n, limit = 5000, 20
	s := NewStatStore(-1, 10)
	for i := range n {
		s.data[uint32(i)] = StatRecord{Views: 10, Clicks: 3, Ftr: 1, Hits: 4, Engagements: 2}
	}

	b.Run("old_GetData+paginate", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			data := s.GetData() // copies all n records
			keys := make([]int, 0, len(data))
			for k := range data {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			page := make(map[int]*StatRecord, limit)
			for _, k := range keys[:limit] {
				page[k] = data[k]
			}
			_ = page
		}
	})
	b.Run("new_GetDataPage", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			s.GetDataPage(limit, 0)
		}
	})
}

// BenchmarkGetData_OldStyle simulates old Statistic.GetData (simple deep copy).
func BenchmarkGetData_OldStyle(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := make(map[int]*StatRecord, n)
			for i := range n {
				data[i] = &StatRecord{Views: 10, Clicks: 3, Ftr: 1}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				copyMap := make(map[int]*StatRecord, len(data))
				for k, v := range data {
					copyMap[k] = &StatRecord{Views: v.Views, Clicks: v.Clicks, Ftr: v.Ftr}
				}
			}
		})
	}
}
