package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrendLineBreakSignal(t *testing.T) {
	t.Run("signals when up trend line is touched", func(t *testing.T) {
		s := TrendLineBreakSignal{
			Price:     1.5,
			Direction: Up,
		}

		prices := make([]Tick, 0)
		trades := Trades{}
		intitialTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

		require.False(t, s.IsSatisfied(prices, trades))

		prices = append(prices, Tick{
			Timestamp: intitialTime,
			Bid:       1.0,
			Ask:       1.1,
		})

		require.False(t, s.IsSatisfied(prices, trades))

		prices = append(prices, Tick{
			Timestamp: intitialTime.Add(1 * time.Minute),
			Bid:       1.5,
			Ask:       1.6,
		})

		require.True(t, s.IsSatisfied(prices, trades))

		prices = append(prices, Tick{
			Timestamp: intitialTime.Add(2 * time.Minute),
			Bid:       1.3,
			Ask:       1.4,
		})

		require.True(t, s.IsSatisfied(prices, trades))
	})

	t.Run("signals when down trend line is touched", func(t *testing.T) {
		s := TrendLineBreakSignal{
			Price:     1.5,
			Direction: Down,
		}

		prices := make([]Tick, 0)
		trades := Trades{}
		intitialTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

		require.False(t, s.IsSatisfied(prices, trades))

		prices = append(prices, Tick{
			Timestamp: intitialTime,
			Bid:       1.4,
			Ask:       1.6,
		})

		require.False(t, s.IsSatisfied(prices, trades))

		prices = append(prices, Tick{
			Timestamp: intitialTime.Add(1 * time.Minute),
			Bid:       1.2,
			Ask:       1.3,
		})

		require.True(t, s.IsSatisfied(prices, trades))

		prices = append(prices, Tick{
			Timestamp: intitialTime.Add(2 * time.Minute),
			Bid:       2.3,
			Ask:       2.4,
		})

		require.True(t, s.IsSatisfied(prices, trades))
	})
}
