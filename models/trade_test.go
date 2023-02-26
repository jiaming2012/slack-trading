package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTrade(t *testing.T) {
	t.Run("trade side", func(t *testing.T) {
		tr := Trade{Volume: 1.0}
		assert.Equal(t, TradeTypeBuy, tr.Side())

		tr = Trade{Volume: -1.0}
		assert.Equal(t, TradeTypeSell, tr.Side())
	})
}
