package models

import (
	"bytes"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReadStatRecords(t *testing.T) {
	data := map[uint32]StatRecord{
		1:    {Views: 10, Clicks: 5, Ftr: 2},
		42:   {Views: 100, Clicks: 50, Ftr: 0},
		1000: {Views: 1, Clicks: 0, Ftr: 0},
	}

	var buf bytes.Buffer
	require.NoError(t, writeStatRecords(&buf, data))

	got, err := readStatRecords(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestWriteReadStatRecords_Empty(t *testing.T) {
	data := map[uint32]StatRecord{}

	var buf bytes.Buffer
	require.NoError(t, writeStatRecords(&buf, data))

	got, err := readStatRecords(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestWriteReadFingerprintRecord(t *testing.T) {
	viewed := roaring.New()
	viewed.Add(1)
	viewed.Add(2)
	viewed.Add(100)

	clicked := roaring.New()
	clicked.Add(1)

	counts := map[uint32]StatRecord{
		1:   {Views: 10, Clicks: 5, Ftr: 2},
		100: {Views: 3, Clicks: 0, Ftr: 0},
	}

	lastSeen := time.Date(2025, 12, 25, 10, 30, 0, 0, time.UTC)

	fr := &FingerprintRecord{
		viewed:   viewed,
		clicked:  clicked,
		counts:   counts,
		lastSeen: lastSeen,
	}

	var buf bytes.Buffer
	require.NoError(t, writeFingerprintRecord(&buf, fr))

	got, err := readFingerprintRecord(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	// Verify bitmaps
	assert.True(t, got.viewed.Contains(1))
	assert.True(t, got.viewed.Contains(2))
	assert.True(t, got.viewed.Contains(100))
	assert.False(t, got.viewed.Contains(3))
	assert.Equal(t, uint64(3), got.viewed.GetCardinality())

	assert.True(t, got.clicked.Contains(1))
	assert.Equal(t, uint64(1), got.clicked.GetCardinality())

	// Verify counts
	assert.Equal(t, counts, got.counts)

	// Verify lastSeen
	assert.Equal(t, lastSeen.UnixNano(), got.lastSeen.UnixNano())
}

func TestWriteReadFingerprintRecord_Empty(t *testing.T) {
	fr := &FingerprintRecord{
		viewed:   roaring.New(),
		clicked:  roaring.New(),
		counts:   map[uint32]StatRecord{},
		lastSeen: time.Time{},
	}

	var buf bytes.Buffer
	require.NoError(t, writeFingerprintRecord(&buf, fr))

	got, err := readFingerprintRecord(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	assert.Equal(t, uint64(0), got.viewed.GetCardinality())
	assert.Equal(t, uint64(0), got.clicked.GetCardinality())
	assert.Empty(t, got.counts)
}

func TestReadFingerprintRecord_TruncatedData(t *testing.T) {
	// Only 4 bytes — not enough for lastSeen (8 bytes)
	_, err := readFingerprintRecord(bytes.NewReader([]byte{0, 0, 0, 0}))
	assert.Error(t, err)
}

func TestReadStatRecords_TruncatedData(t *testing.T) {
	// Says 1 record but provides no data
	data := []byte{1, 0, 0, 0} // count=1
	_, err := readStatRecords(bytes.NewReader(data))
	assert.Error(t, err)
}

func TestWriteReadString(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeString(&buf, "hello"))

	got, err := readString(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

func TestWriteReadString_Empty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeString(&buf, ""))

	got, err := readString(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestWriteReadBitmap(t *testing.T) {
	bm := roaring.New()
	bm.Add(42)
	bm.Add(1000000)

	var buf bytes.Buffer
	require.NoError(t, writeBitmap(&buf, bm))

	got, err := readBitmap(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.True(t, got.Contains(42))
	assert.True(t, got.Contains(1000000))
	assert.Equal(t, uint64(2), got.GetCardinality())
}
