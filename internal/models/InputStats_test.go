package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputStats_UnmarshalFull(t *testing.T) {
	raw := `{"f":"abc123","c":["1","2"],"v":["3","4","5"],"ch":"news"}`
	var is InputStats
	require.NoError(t, json.Unmarshal([]byte(raw), &is))
	assert.Equal(t, "abc123", is.Fingerprint)
	assert.Equal(t, []string{"1", "2"}, is.Clicks)
	assert.Equal(t, []string{"3", "4", "5"}, is.Views)
	assert.Equal(t, "news", is.Channel)
}

func TestInputStats_UnmarshalEmpty(t *testing.T) {
	raw := `{}`
	var is InputStats
	require.NoError(t, json.Unmarshal([]byte(raw), &is))
	assert.Empty(t, is.Fingerprint)
	assert.Nil(t, is.Clicks)
	assert.Nil(t, is.Views)
	assert.Empty(t, is.Channel)
}
