package models

import "math"

func UnrealizedPL(vwap Vwap, vol Volume, tick Tick) float64 {
	if vol > 0 {
		return (tick.Bid - float64(vwap)) * float64(vol)
	} else if vol < 0 {
		return (float64(vwap) - tick.Ask) * math.Abs(float64(vol))
	} else {
		return 0
	}
}
