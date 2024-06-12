package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"slack-trading/src/eventmodels"
)

func TestCreateTag(t *testing.T) {
	t.Run("Encode Tag", func(t *testing.T) {
		signal := eventmodels.SignalName("supertrend-4h-1h_stoch_rsi_15m_up")
		tag := EncodeTag(signal, 9.53, 21.45)
		assert.Equal(t, tag, "supertrend--4h--1h-stoch-rsi-15m-up---9-53---21-45")
	})

	t.Run("Decode tag", func(t *testing.T) {
		tag := "supertrend--4h--1h-stoch-rsi-15m-up---9-53---21-45"
		signal, expectedProfit, requestedPrc, err := DecodeTag(tag)
		assert.NoError(t, err)
		assert.Equal(t, eventmodels.SignalName("supertrend-4h-1h_stoch_rsi_15m_up"), signal)
		assert.Equal(t, 9.53, expectedProfit)
		assert.Equal(t, 21.45, requestedPrc)
	})
}
