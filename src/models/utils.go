package models

import "math"

func UnrealizedPL(vwap Vwap, vol Volume, price float64) float64 {
	if vol > 0 {
		return (price - float64(vwap)) * float64(vol)
	} else if vol < 0 {
		return (float64(vwap) - price) * math.Abs(float64(vol))
	} else {
		return 0
	}
}
