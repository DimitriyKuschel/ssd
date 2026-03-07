package models

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
)

var byteOrder = binary.LittleEndian

// writeString writes a uint16 length-prefixed UTF-8 string.
func writeString(w io.Writer, s string) error {
	if err := binary.Write(w, byteOrder, uint16(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

// readString reads a uint16 length-prefixed UTF-8 string.
func readString(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, byteOrder, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// writeStatRecords writes a map[uint32]StatRecord in binary format.
// Format: count(uint32) + for each: id(uint32) views(int32) clicks(int32) ftr(int32)
func writeStatRecords(w io.Writer, data map[uint32]StatRecord) error {
	if err := binary.Write(w, byteOrder, uint32(len(data))); err != nil {
		return err
	}
	for id, rec := range data {
		if err := binary.Write(w, byteOrder, id); err != nil {
			return err
		}
		if err := binary.Write(w, byteOrder, int32(rec.Views)); err != nil {
			return err
		}
		if err := binary.Write(w, byteOrder, int32(rec.Clicks)); err != nil {
			return err
		}
		if err := binary.Write(w, byteOrder, int32(rec.Ftr)); err != nil {
			return err
		}
	}
	return nil
}

// readStatRecords reads a map[uint32]StatRecord from binary format.
func readStatRecords(r io.Reader) (map[uint32]StatRecord, error) {
	var count uint32
	if err := binary.Read(r, byteOrder, &count); err != nil {
		return nil, err
	}
	data := make(map[uint32]StatRecord, count)
	for i := uint32(0); i < count; i++ {
		var id uint32
		var views, clicks, ftr int32
		if err := binary.Read(r, byteOrder, &id); err != nil {
			return nil, err
		}
		if err := binary.Read(r, byteOrder, &views); err != nil {
			return nil, err
		}
		if err := binary.Read(r, byteOrder, &clicks); err != nil {
			return nil, err
		}
		if err := binary.Read(r, byteOrder, &ftr); err != nil {
			return nil, err
		}
		data[id] = StatRecord{Views: int(views), Clicks: int(clicks), Ftr: int(ftr)}
	}
	return data, nil
}

// writeBitmap writes a Roaring Bitmap as uint32 length + MarshalBinary bytes.
func writeBitmap(w io.Writer, bm *roaring.Bitmap) error {
	buf, err := bm.MarshalBinary()
	if err != nil {
		return err
	}
	if err := binary.Write(w, byteOrder, uint32(len(buf))); err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

// readBitmap reads a Roaring Bitmap from uint32 length + binary data.
func readBitmap(r io.Reader) (*roaring.Bitmap, error) {
	var length uint32
	if err := binary.Read(r, byteOrder, &length); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	bm := roaring.New()
	if err := bm.UnmarshalBinary(buf); err != nil {
		return nil, fmt.Errorf("roaring unmarshal: %w", err)
	}
	return bm, nil
}

// writeFingerprintRecord writes a FingerprintRecord in binary format.
// Caller must hold fr.mu.
func writeFingerprintRecord(w io.Writer, fr *FingerprintRecord) error {
	// lastSeen as Unix nanoseconds
	if err := binary.Write(w, byteOrder, fr.lastSeen.UnixNano()); err != nil {
		return err
	}
	// viewed bitmap
	if err := writeBitmap(w, fr.viewed); err != nil {
		return err
	}
	// clicked bitmap
	if err := writeBitmap(w, fr.clicked); err != nil {
		return err
	}
	// counts
	return writeStatRecords(w, fr.counts)
}

// readFingerprintRecord reads a FingerprintRecord from binary format,
// creating the record directly without going through dataToFingerprintRecord.
func readFingerprintRecord(r io.Reader) (*FingerprintRecord, error) {
	var nanos int64
	if err := binary.Read(r, byteOrder, &nanos); err != nil {
		return nil, err
	}

	viewed, err := readBitmap(r)
	if err != nil {
		return nil, err
	}

	clicked, err := readBitmap(r)
	if err != nil {
		return nil, err
	}

	counts, err := readStatRecords(r)
	if err != nil {
		return nil, err
	}

	return &FingerprintRecord{
		viewed:   viewed,
		clicked:  clicked,
		counts:   counts,
		lastSeen: time.Unix(0, nanos),
	}, nil
}
