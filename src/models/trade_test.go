package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTrade(t *testing.T) {
	t.Run("trade side", func(t *testing.T) {
		tr := Trade{RequestedVolume: 1.0}
		assert.Equal(t, TradeTypeBuy, tr.Side())

		tr = Trade{RequestedVolume: -1.0}
		assert.Equal(t, TradeTypeSell, tr.Side())
	})

	t.Run("validate volume is non zero", func(t *testing.T) {
		tr := Trade{RequestedVolume: 0.0, StopLoss: 1.5}
		err := tr.Validate()
		assert.ErrorIs(t, err, TradeVolumeIsZeroErr)
	})

	t.Run("validate stop loss", func(t *testing.T) {
		t.Run("passes validation", func(t *testing.T) {
			tr := Trade{RequestedVolume: 1.0, RequestedPrice: 1.0, StopLoss: 0.5}
			err := tr.Validate()
			assert.Nil(t, err)
		})

		t.Run("errors if trade has no stop loss", func(t *testing.T) {
			tr := Trade{RequestedVolume: 1.0}
			err := tr.Validate()
			assert.ErrorIs(t, err, NoStopLossErr)
		})

		t.Run("errors if buy order has stop loss above current price", func(t *testing.T) {
			tr := Trade{RequestedVolume: 1.0, RequestedPrice: 1.0, StopLoss: 1.5}
			err := tr.Validate()
			assert.ErrorContains(t, err, InvalidStopLossErr.Error())
		})

		t.Run("errors if sell order has stop loss below current price", func(t *testing.T) {
			tr := Trade{RequestedVolume: -1.0, RequestedPrice: 1.0, StopLoss: 0.5}
			err := tr.Validate()
			assert.ErrorContains(t, err, InvalidStopLossErr.Error())
		})
	})
}
