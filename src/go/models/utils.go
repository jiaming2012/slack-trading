package models

import "math"

func UnrealizedPL(vwap Vwap, vol Volume, tick Tick) float64 {
	// Adapted from legacy bid/ask ticks to the canonical single-price Tick model
	// (reconcile-models-packages: bid/ask -> Price).
	if vol > 0 {
		return (tick.Price - float64(vwap)) * float64(vol)
	} else if vol < 0 {
		return (float64(vwap) - tick.Price) * math.Abs(float64(vol))
	} else {
		return 0
	}
}
