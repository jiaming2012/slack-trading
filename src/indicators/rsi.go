package indicators

import (
	"math"
	"slack-trading/src/models"
)

type Rsi struct {
	prevAvgGain *float64
	prevAvgLoss *float64
	closes      []float64
	Period      int
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func (r *Rsi) deriveRS() float64 {
	if r.prevAvgGain != nil {
		curPrice := r.closes[len(r.closes)-1]
		prevPrice := r.closes[len(r.closes)-2]
		delta := curPrice - prevPrice

		var deltaGain, deltaLoss float64
		if delta > 0 {
			deltaGain = delta
			deltaLoss = 0.0
		} else {
			deltaGain = 0.0
			deltaLoss = math.Abs(delta)
		}

		avgGain := ((*r.prevAvgGain)*(float64(r.Period)-1.0) + deltaGain) / float64(r.Period)
		avgLoss := ((*r.prevAvgLoss)*(float64(r.Period)-1.0) + deltaLoss) / float64(r.Period)

		if avgLoss == 0 {
			return 100
		}

		r.prevAvgGain = &avgGain
		r.prevAvgLoss = &avgLoss
		return avgGain / avgLoss
	}

	gains := make([]float64, r.Period+1)
	losses := make([]float64, r.Period+1)

	prevPrice := r.closes[0]
	for i, price := range r.closes {
		delta := price - prevPrice
		if delta > 0 {
			gains[i] = delta
			losses[i] = 0
		} else {
			gains[i] = 0
			losses[i] = math.Abs(delta)
		}

		prevPrice = price
	}

	avgGain := average(gains[1:])
	avgLoss := average(losses[1:])
	r.prevAvgGain = &avgGain
	r.prevAvgLoss = &avgLoss

	if avgLoss == 0 {
		return 100
	}

	return avgGain / avgLoss
}

func (r *Rsi) Update(c models.Candle) float64 {
	if len(r.closes) < r.Period {
		r.closes = append(r.closes, c.Close)
		return 0
	}

	r.closes = append(r.closes, c.Close)

	rs := r.deriveRS()

	r.closes = r.closes[1:]

	if rs == 0 {
		return 0
	}

	return 100 - (100 / (1 + rs))
}

func NewRsi(period int) *Rsi {
	return &Rsi{
		Period: period,
	}
}
