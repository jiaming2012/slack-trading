package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ReadOptionChainExpectedValue struct {
	Lookback time.Duration
	Signal   string
}

type ReadOptionChainRequest struct {
	Symbol                    StockSymbol  `json:"symbol"`
	OptionTypes               []OptionType `json:"optionTypes"`
	ExpirationsInDays         []int        `json:"expirationsInDays"`
	MinDistanceBetweenStrikes float64      `json:"minDistanceBetweenStrikes"`
	MaxNoOfStrikes            int          `json:"maxNoOfStrikes"`
	EV                        *ReadOptionChainExpectedValue
}

func (o *ReadOptionChainRequest) Validate(r *http.Request) error {
	if o.Symbol == "" {
		return fmt.Errorf("ReadOptionChainRequest: Validate: symbol is required")
	}

	if len(o.ExpirationsInDays) == 0 {
		return fmt.Errorf("ReadOptionChainRequest: Validate: expirationsInDays is required")
	}

	if o.MinDistanceBetweenStrikes == 0 {
		return fmt.Errorf("ReadOptionChainRequest: Validate: minDistanceBetweenStrikes is required")
	}

	if o.MaxNoOfStrikes == 0 {
		return fmt.Errorf("ReadOptionChainRequest: Validate: maxNoOfStrikes is required")
	}

	if len(o.OptionTypes) == 0 {
		return fmt.Errorf("ReadOptionChainRequest: Validate: optionTypes is required")
	}

	for _, optionType := range o.OptionTypes {
		if err := optionType.Validate(); err != nil {
			return fmt.Errorf("ReadOptionChainRequest: Validate: %w", err)
		}
	}

	if o.EV != nil {
		if o.EV.Lookback == 0 {
			return fmt.Errorf("ReadOptionChainRequest: Validate: EV.Lookback is required")
		}

		if o.EV.Signal == "" {
			return fmt.Errorf("ReadOptionChainRequest: Validate: EV.Signal is required")
		}
	}

	return nil
}

func (o *ReadOptionChainRequest) ParseHTTPRequest(r *http.Request) error {
	jsonDecoder := json.NewDecoder(r.Body)
	if err := jsonDecoder.Decode(o); err != nil {
		return fmt.Errorf("ReadOptionChainRequest: ParseHTTPRequest: decode: %w", err)
	}

	// parse query params
	query := r.URL.Query()
	if ev := query.Get("ev"); ev != "" {
		expectedValue := &ReadOptionChainExpectedValue{}

		if lookback := query.Get("lookback"); lookback != "" {
			months, days, _, err := ParseDuration(lookback)
			if err != nil {
				return fmt.Errorf("ReadOptionChainRequest: ParseHTTPRequest: parse lookback: %w", err)
			}

			now := time.Now()
			future := now.AddDate(0, months, days)
			duration := future.Sub(now)
			expectedValue.Lookback = duration
		}

		if signal := query.Get("signal"); signal != "" {
			expectedValue.Signal = signal
		}

		o.EV = expectedValue
	}

	return nil
}
