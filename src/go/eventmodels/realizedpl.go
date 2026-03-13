package eventmodels

import (
	"fmt"
	"math"
)

type RealizedPL float64

func (realizedPL RealizedPL) Validate() error {
	if math.IsNaN(float64(realizedPL)) {
		return fmt.Errorf("realizedPL.Validate: NaN is not a valid value")
	}

	if math.IsInf(float64(realizedPL), 0) {
		return fmt.Errorf("realizedPL.Validate: +/- Inf is not a valid value")
	}

	return nil
}
