package eventmodels

import "fmt"

type CloseTradeRequestV2 struct {
	BaseRequestEvent
	Trade     *Trade
	Timeframe *int
	Percent   float64
	Reason    string
}

func (r *CloseTradeRequestV2) Validate() error {
	if r.Trade == nil {
		return fmt.Errorf("CloseTradesRequest.Validate: trade not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if r.Percent < 0 || r.Percent > 1 {
		return InvalidClosePercentErr
	}

	return nil
}
