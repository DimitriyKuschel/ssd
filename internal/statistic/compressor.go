package statistic

import (
	"fmt"
	"github.com/klauspost/compress/zstd"
	"ssd/internal/statistic/interfaces"
)

type ZstdCompression struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

func (z *ZstdCompression) Compress(val []byte) ([]byte, error) {
	return z.encoder.EncodeAll(val, make([]byte, 0, len(val)/2)), nil
}

func (z *ZstdCompression) Decompress(val []byte) ([]byte, error) {
	return z.decoder.DecodeAll(val, nil)
}

func NewZstdCompressor() (interfaces.CompressorInterface, error) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	return &ZstdCompression{encoder: encoder, decoder: decoder}, nil
}
