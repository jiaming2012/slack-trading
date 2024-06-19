package indicators

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/models"
	"github.com/montanaflynn/stats"
)

type BollingerBands struct {
	SmaPeriod         int
	StandardDeviation float64
	typicalPrice      []float64
}

type BollingerBandsStats struct {
	Upper         float64
	Lower         float64
	MovingAverage float64
}

func (b *BollingerBands) Update(c models.Candle) (bool, BollingerBandsStats, error) {
	typicalPrice := (c.High + c.Low + c.Close) / 3.0
	if len(b.typicalPrice) < b.SmaPeriod {
		b.typicalPrice = append(b.typicalPrice, typicalPrice)
		return false, BollingerBandsStats{}, nil
	}

	b.typicalPrice = append(b.typicalPrice[1:], typicalPrice)

	movingAverage, err := stats.Mean(b.typicalPrice)
	if err != nil {
		return false, BollingerBandsStats{}, fmt.Errorf("failed to caculate mean: %v", err)
	}

	sd, err := stats.StandardDeviation(b.typicalPrice)
	if err != nil {
		return false, BollingerBandsStats{}, fmt.Errorf("failed to caculate the standard deviation: %v", err)
	}

	return true, BollingerBandsStats{
		Upper:         movingAverage + (b.StandardDeviation * sd),
		Lower:         movingAverage - (b.StandardDeviation * sd),
		MovingAverage: movingAverage,
	}, nil
}

func NewBollingerBands(smaPeriod int, standardDeviation float64) *BollingerBands {
	return &BollingerBands{
		SmaPeriod:         smaPeriod,
		StandardDeviation: standardDeviation,
	}
}
