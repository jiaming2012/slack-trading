package eventmodels

import (
	"testing"
	"time"
)

func TestTradingViewCandles(t *testing.T) {
	c1 := &TradingViewCandle{
		Timestamp: time.Date(2022, time.January, 1, 12, 0, 0, 0, time.UTC),
		Close:     1,
		UpTrend:   1,
	}

	c2 := &TradingViewCandle{
		Timestamp: time.Date(2022, time.January, 2, 12, 0, 0, 0, time.UTC),
		Close:     2,
		UpTrend:   2,
	}

	c3 := &TradingViewCandle{
		Timestamp: time.Date(2022, time.January, 3, 12, 0, 0, 0, time.UTC),
		Close:     3,
		UpTrend:   3,
	}

	noCandles := TradingViewCandles{}

	singleCandle := TradingViewCandles{
		c1,
	}

	multipleCandles := TradingViewCandles{
		c1,
		c2,
		c3,
	}

	t.Run("find closest candle", func(t *testing.T) {
		closestCandle := singleCandle.FindClosestCandleBeforeOrAt(time.Date(2022, time.January, 1, 12, 0, 0, 0, time.UTC))
		if closestCandle != c1 {
			t.Errorf("expected closest candle to be c1, got %v", closestCandle)
		}
	})

	t.Run("nil if no trading candles present", func(t *testing.T) {
		closestCandle := noCandles.FindClosestCandleBeforeOrAt(time.Date(2022, time.January, 1, 12, 0, 0, 0, time.UTC))
		if closestCandle != nil {
			t.Errorf("expected closest candle to be nil, got %v", closestCandle)
		}
	})

	t.Run("timestamp before c2 return c1", func(t *testing.T) {
		closestCandle := multipleCandles.FindClosestCandleBeforeOrAt(time.Date(2022, time.January, 2, 11, 0, 0, 0, time.UTC))
		if closestCandle != c1 {
			t.Errorf("expected closest candle to be c1, got %v", closestCandle)
		}
	})

	t.Run("timestamp at c2 return c2", func(t *testing.T) {
		closestCandle := multipleCandles.FindClosestCandleBeforeOrAt(time.Date(2022, time.January, 2, 12, 0, 0, 0, time.UTC))
		if closestCandle != c2 {
			t.Errorf("expected closest candle to be c2, got %v", closestCandle)
		}
	})

	t.Run("timestamp after c2 return c2", func(t *testing.T) {
		closestCandle := multipleCandles.FindClosestCandleBeforeOrAt(time.Date(2022, time.January, 2, 13, 0, 0, 0, time.UTC))
		if closestCandle != c2 {
			t.Errorf("expected closest candle to be c2, got %v", closestCandle)
		}
	})

	t.Run("timestamp after last candle return last candle", func(t *testing.T) {
		closestCandle := multipleCandles.FindClosestCandleBeforeOrAt(time.Date(2022, time.January, 4, 12, 0, 0, 0, time.UTC))
		if closestCandle != c3 {
			t.Errorf("expected closest candle to be c3, got %v", closestCandle)
		}
	})
}
