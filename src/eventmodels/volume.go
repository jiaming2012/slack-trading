package eventmodels

import (
	"fmt"
	"math"
)

type Volume float64

func (volume Volume) Validate() error {
	if math.IsNaN(float64(volume)) {
		return fmt.Errorf("vwap.volume: NaN is not a valid value")
	}

	if math.IsInf(float64(volume), 0) {
		return fmt.Errorf("vwap.volume: +/- Inf is not a valid value")
	}

	return nil
}
