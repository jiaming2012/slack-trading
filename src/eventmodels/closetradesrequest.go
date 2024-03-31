package eventmodels

import "fmt"

type CloseTradesRequest struct {
	BaseRequestEvent2
	Strategy        *Strategy
	Timeframe       *int
	PriceLevelIndex int
	Percent         float64
	Reason          string
}

func (r *CloseTradesRequest) Validate() error {
	if r.Strategy == nil {
		return fmt.Errorf("CloseTradesRequest.Validate: strategy not set")
	}

	if r.Timeframe != nil && *r.Timeframe <= 0 {
		return InvalidTimeframeErr
	}

	if r.PriceLevelIndex < 0 {
		return fmt.Errorf("CloseTradesRequest.Validate: found %v: %w", r.PriceLevelIndex, InvalidPriceLevelIndexErr)
	}

	if r.Percent < 0 || r.Percent > 1 {
		return InvalidClosePercentErr
	}

	if r.Reason == "" {
		return fmt.Errorf("CloseTradesRequest.Validate: reason not set")
	}

	return nil
}

func NewCloseTradesRequest(strategy *Strategy, timeframe *int, priceLevelIndex int, percent float64, reason string) (*CloseTradesRequest, error) {
	closeReq := &CloseTradesRequest{Strategy: strategy, Timeframe: timeframe, PriceLevelIndex: priceLevelIndex, Percent: percent, Reason: reason}

	if err := closeReq.Validate(); err != nil {
		return nil, fmt.Errorf("NewCloseTradesRequest validation failed: %w", err)
	}

	return closeReq, nil
}
