package eventmodels

import (
	"fmt"
	"math"
)

type Vwap float64

func (vwap Vwap) Validate() error {
	if math.IsNaN(float64(vwap)) {
		return fmt.Errorf("vwap.Validate: NaN is not a valid value")
	}

	if math.IsInf(float64(vwap), 0) {
		return fmt.Errorf("vwap.Validate: +/- Inf is not a valid value")
	}

	return nil
}
