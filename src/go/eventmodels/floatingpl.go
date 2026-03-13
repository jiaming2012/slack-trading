package eventmodels

import (
	"fmt"
	"math"
)

type FloatingPL float64

func (floatingPL FloatingPL) Validate() error {
	if math.IsNaN(float64(floatingPL)) {
		return fmt.Errorf("vwap.Validate: NaN is not a valid value")
	}

	if math.IsInf(float64(floatingPL), 0) {
		return fmt.Errorf("vwap.Validate: +/- Inf is not a valid value")
	}

	return nil
}
