package indicators

import (
	"github.com/stretchr/testify/assert"
	"math"
	"slack-trading/src/models"
	"testing"
)

func TestBollingerBands(t *testing.T) {
	candles := models.Candles{
		Period: 5,
		Data: []models.Candle{
			{
				High:  90.70,
				Low:   90.70,
				Close: 90.70,
			},
			{
				High:  92.90,
				Low:   92.90,
				Close: 92.90,
			},
			{
				High:  92.98,
				Low:   92.98,
				Close: 92.98,
			},
			{
				High:  91.80,
				Low:   91.80,
				Close: 91.80,
			},
			{
				High:  92.66,
				Low:   92.66,
				Close: 92.66,
			},
			{
				High:  92.68,
				Low:   92.68,
				Close: 92.68,
			},
			{
				High:  92.30,
				Low:   92.30,
				Close: 92.30,
			},
			{
				High:  92.77,
				Low:   92.77,
				Close: 92.77,
			},
			{
				High:  92.54,
				Low:   92.54,
				Close: 92.54,
			},
			{
				High:  92.95,
				Low:   92.95,
				Close: 92.95,
			},
			{
				High:  93.20,
				Low:   93.20,
				Close: 93.20,
			},
			{
				High:  91.07,
				Low:   91.07,
				Close: 91.07,
			},
			{
				High:  89.83,
				Low:   89.83,
				Close: 89.83,
			},
			{
				High:  89.74,
				Low:   89.74,
				Close: 89.74,
			},
			{
				High:  90.40,
				Low:   90.40,
				Close: 90.40,
			},
			{
				High:  90.74,
				Low:   90.74,
				Close: 90.74,
			},
			{
				High:  88.02,
				Low:   88.02,
				Close: 88.02,
			},
			{
				High:  88.09,
				Low:   88.09,
				Close: 88.09,
			},
			{
				High:  88.84,
				Low:   88.84,
				Close: 88.84,
			},
			{
				High:  90.78,
				Low:   90.78,
				Close: 90.78,
			},
			{
				High:  90.54,
				Low:   90.54,
				Close: 90.54,
			},
			{
				High:  91.39,
				Low:   91.39,
				Close: 91.39,
			},
			{
				High:  90.65,
				Low:   90.65,
				Close: 90.65,
			},
		},
	}

	t.Run("Calculate bands", func(t *testing.T) {
		var stats BollingerBandsStats
		bollinger := NewBollingerBands(20, 2.0)
		for _, c := range candles.Data {
			_, _stats, err := bollinger.Update(c)
			assert.NoError(t, err)
			stats = _stats
		}

		assert.Equal(t, 91.0, math.Floor(stats.MovingAverage*10)/10)
		assert.Equal(t, 94.1, math.Floor(stats.Upper*10)/10)
		assert.Equal(t, 87.9, math.Floor(stats.Lower*10)/10)
	})
}
