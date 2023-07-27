package notifier

import (
	"github.com/stretchr/testify/assert"
	"slack-trading/src/eventconsumers/unpure/notifier/models"
	"testing"
	"time"
)

func TestRealizedProfitLossEvent(t *testing.T) {
	t.Run("", func(t *testing.T) {
		m := NewMockNotifier()
		ev := models.RealizedProfitLossEvent{
			Profit: 99.873,
		}
		expected := "Realized PL: $99.87"

		assert.Equal(t, expected, m.CompileRealizedProfitLossEvent(ev))
	})
}

func TestTradeFulfilledEvent(t *testing.T) {
	t.Run("new buy order", func(t *testing.T) {
		ts := time.Date(2022, 01, 02, 12, 30, 30, 0, time.UTC)
		m := NewMockNotifier()
		ev := models.TradeFulfilledEvent{
			Timestamp: ts,
			Symbol:    "BTCUSD",
			Volume:    1.5,
			Price:     30023.243,
		}
		expected := "2022-01-02 12:30:30 +0000 UTC: +1.50000000 BTCUSD @ $30023.24"

		assert.Equal(t, expected, m.CompileTradeFulfilledEvent(ev))
	})
}
