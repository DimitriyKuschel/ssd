package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_JSONRoundtrip(t *testing.T) {
	original := Storage{
		Channels: map[string]*ChannelData{
			"default": {
				TrendStats: map[int]*StatRecord{
					1: {Views: 10, Clicks: 2, Ftr: 0},
				},
				PersonalStats: map[string]*Statistic{
					"fp1": {Data: map[int]*StatRecord{1: {Views: 5, Clicks: 1}}},
				},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Storage
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Len(t, restored.Channels, 1)
	ch := restored.Channels["default"]
	require.NotNil(t, ch)
	assert.Equal(t, 10, ch.TrendStats[1].Views)
	assert.Equal(t, 5, ch.PersonalStats["fp1"].Data[1].Views)
}

func TestStorage_NilFields(t *testing.T) {
	raw := `{"channels":{"ch1":{}}}`
	var s Storage
	require.NoError(t, json.Unmarshal([]byte(raw), &s))
	assert.Nil(t, s.Channels["ch1"].TrendStats)
	assert.Nil(t, s.Channels["ch1"].PersonalStats)
}
