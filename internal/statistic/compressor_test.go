package statistic

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZstdCompression_Roundtrip(t *testing.T) {
	c, err := NewZstdCompressor()
	require.NoError(t, err)

	original := []byte(`{"key":"value","number":42}`)
	compressed, err := c.Compress(original)
	require.NoError(t, err)
	assert.NotEqual(t, original, compressed)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)
}

func TestZstdCompression_EmptyData(t *testing.T) {
	c, err := NewZstdCompressor()
	require.NoError(t, err)

	compressed, err := c.Compress([]byte{})
	require.NoError(t, err)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Empty(t, decompressed)
}

func TestZstdCompression_LargeData(t *testing.T) {
	c, err := NewZstdCompressor()
	require.NoError(t, err)

	original := bytes.Repeat([]byte("abcdefghij"), 100_000) // 1MB
	compressed, err := c.Compress(original)
	require.NoError(t, err)
	// Repetitive data should compress well
	assert.Less(t, len(compressed), len(original)/2)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)
}

func TestZstdCompression_DecompressInvalidData(t *testing.T) {
	c, err := NewZstdCompressor()
	require.NoError(t, err)

	_, err = c.Decompress([]byte("not valid zstd data"))
	assert.Error(t, err)
}

func TestZstdCompression_DecompressRandomBytes(t *testing.T) {
	c, err := NewZstdCompressor()
	require.NoError(t, err)

	_, err = c.Decompress([]byte{0xff, 0xfe, 0xfd, 0xfc, 0x00, 0x01})
	assert.Error(t, err)
}

func TestNewZstdCompressor_Success(t *testing.T) {
	c, err := NewZstdCompressor()
	require.NoError(t, err)
	require.NotNil(t, c)
}
