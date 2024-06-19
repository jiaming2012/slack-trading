package indicators

import (
	"math"
	"testing"

	"github.com/jiaming2012/slack-trading/src/models"
	"github.com/stretchr/testify/assert"
)

const equalityThreshold = 1e-2

func TestRsi(t *testing.T) {
	t.Run("example rsi", func(t *testing.T) {
		// example taken from https://blog.quantinsti.com/rsi-indicator/
		rsi := NewRsi(14)
		candles := []models.Candle{
			{
				Close: 283.46,
			},
			{
				Close: 280.69,
			},
			{
				Close: 285.48,
			},
			{
				Close: 294.08,
			},
			{
				Close: 293.90,
			},
			{
				Close: 299.92,
			},
			{
				Close: 301.15,
			},
			{
				Close: 284.45,
			},
			{
				Close: 294.09,
			},
			{
				Close: 302.77,
			},
			{
				Close: 301.97,
			},
			{
				Close: 306.85,
			},
			{
				Close: 305.02,
			},
			{
				Close: 301.06,
			},
			{
				Close: 291.97,
			},
		}

		for i, c := range candles {
			val := rsi.Update(c)
			if i < len(candles)-1 {
				assert.Equal(t, 0.0, val)
			} else {
				expected := 55.37
				diff := math.Abs(val - expected)
				assert.Less(t, diff, equalityThreshold)
			}
		}

		// add new candle
		val := rsi.Update(models.Candle{
			Close: 284.18,
		})

		expected := 50.07
		diff := math.Abs(val - expected)
		assert.Less(t, diff, equalityThreshold)

		// add another new candle
		val = rsi.Update(models.Candle{
			Close: 286.48,
		})

		expected = 51.55
		diff = math.Abs(val - expected)
		assert.Less(t, diff, equalityThreshold)

		// add yet another new candle
		val = rsi.Update(models.Candle{
			Close: 284.54,
		})

		expected = 50.20
		diff = math.Abs(val - expected)
		assert.Less(t, diff, equalityThreshold)
	})

	t.Run("too few candles", func(t *testing.T) {
		rsi := NewRsi(14)
		val := rsi.Update(models.Candle{Close: 100.0})
		assert.Equal(t, val, 0.0)
	})

	t.Run("all losers", func(t *testing.T) {
		candles := []models.Candle{
			{
				Close: 10.0,
			},
			{
				Close: 9.0,
			},
			{
				Close: 5.0,
			},
		}

		rsi := NewRsi(2)
		var val float64
		for _, c := range candles {
			val = rsi.Update(c)
		}

		assert.Equal(t, 0.0, val)
	})

	t.Run("all winners", func(t *testing.T) {
		candles := []models.Candle{
			{
				Close: 10.0,
			},
			{
				Close: 11.0,
			},
			{
				Close: 15.0,
			},
		}

		rsi := NewRsi(2)
		var val float64
		for _, c := range candles {
			val = rsi.Update(c)
		}

		expected := 99.0
		diff := math.Abs(val - expected)
		assert.Less(t, diff, equalityThreshold)
	})
}
