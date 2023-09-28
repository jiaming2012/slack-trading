package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestSignalConstraints_Validate(t *testing.T) {
	t.Run("duplicate names not allowed", func(t *testing.T) {
		c1 := NewExitSignalConstraint("c1", nil)
		c2 := NewExitSignalConstraint("c2", nil)
		constraints := SignalConstraints{c1, c2}
		assert.NoError(t, constraints.Validate())

		c3 := NewExitSignalConstraint("c1", nil)
		constraints = SignalConstraints{c1, c2, c3}
		assert.Error(t, constraints.Validate())
	})
}

func TestPriceLevelProfitLossAboveZeroConstraint(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("true when trades pl > 0", func(t *testing.T) {
		reqPrice := 1000.0
		sl := reqPrice - 500.0
		reqVol := 1.0

		priceLevel := PriceLevel{
			Strategy:             nil,
			Price:                0,
			MinimumTradeDistance: 0,
			MaxNoOfTrades:        0,
			AllocationPercent:    0,
			Trades:               &Trades{},
			StopLoss:             0,
			mutex:                sync.Mutex{},
		}

		t1, _, err := NewOpenTrade(id, TradeTypeBuy, "symbol", nil, ts, reqPrice, reqVol, sl, nil)

		err = priceLevel.Add(t1, reqPrice, reqVol)

		assert.NoError(t, err)

		params := map[string]interface{}{
			"tick": Tick{
				Timestamp: time.Time{},
				Bid:       reqPrice,
				Ask:       reqPrice,
			},
		}

		res, err := PriceLevelProfitLossAboveZeroConstraint(&priceLevel, nil, params)
		assert.NoError(t, err)
		assert.False(t, res)

		params["tick"] = Tick{
			Bid: reqPrice + 100.0,
			Ask: reqPrice + 100.0,
		}

		res, err = PriceLevelProfitLossAboveZeroConstraint(&priceLevel, nil, params)
		assert.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("false when no trades have been placed", func(t *testing.T) {
		priceLevel := PriceLevel{
			Strategy:             nil,
			Price:                0,
			MinimumTradeDistance: 0,
			MaxNoOfTrades:        0,
			AllocationPercent:    0,
			Trades:               nil,
			StopLoss:             0,
			mutex:                sync.Mutex{},
		}

		exitCondition := ExitCondition{
			ExitSignals:            nil,
			ReentrySignals:         nil,
			Constraints:            nil,
			LevelIndex:             0,
			MaxTriggerCount:        nil,
			TriggerCount:           0,
			ClosePercent:           0,
			AwaitingReentrySignals: false,
		}

		params := map[string]interface{}{
			"tick": Tick{
				Timestamp: time.Time{},
				Bid:       1.0,
				Ask:       1.0,
			},
		}

		res, err := PriceLevelProfitLossAboveZeroConstraint(&priceLevel, &exitCondition, params)
		assert.NoError(t, err)
		assert.False(t, res)
	})
}
