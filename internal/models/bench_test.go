package models

import (
	"fmt"
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
			for i := 0; i < b.N; i++ {
				fr.buildData()
			}
		})
	}
}

// BenchmarkGetData_OldStyle simulates old Statistic.GetData (simple deep copy).
func BenchmarkGetData_OldStyle(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			data := make(map[int]*StatRecord, n)
			for i := 0; i < n; i++ {
				data[i] = &StatRecord{Views: 10, Clicks: 3, Ftr: 1}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				copyMap := make(map[int]*StatRecord, len(data))
				for k, v := range data {
					copyMap[k] = &StatRecord{Views: v.Views, Clicks: v.Clicks, Ftr: v.Ftr}
				}
			}
		})
	}
}
