package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ReadOptionChainExpectedValue struct {
	StartsAt time.Time
	EndsAt   time.Time
	Signal   string
}

type ReadOptionChainRequest struct {
	Symbol                             StockSymbol  `json:"symbol"`
	OptionTypes                        []OptionType `json:"optionTypes"`
	ExpirationsInDays                  []int        `json:"expirationsInDays"`
	MinDistanceBetweenStrikes          *float64     `json:"minDistanceBetweenStrikes"`
	MinStandardDeviationBetweenStrikes *float64     `json:"minStandardDeviationBetweenStrikes"`
	MaxNoOfStrikes                     int          `json:"maxNoOfStrikes"`
	EV                                 *ReadOptionChainExpectedValue
}

func (o *ReadOptionChainRequest) Validate(r *http.Request) error {
	if o.Symbol == "" {
		return fmt.Errorf("ReadOptionChainRequest: Validate: symbol is required")
	}

	if len(o.ExpirationsInDays) == 0 {
		return fmt.Errorf("ReadOptionChainRequest: Validate: expirationsInDays is required")
	}

	if o.MinDistanceBetweenStrikes == nil && o.MinStandardDeviationBetweenStrikes == nil {
		return fmt.Errorf("ReadOptionChainRequest: Validate: minDistanceBetweenStrikes or minStandardDeviationBetweenStrikes is required")
	}

	if o.MinDistanceBetweenStrikes != nil && o.MinStandardDeviationBetweenStrikes != nil {
		return fmt.Errorf("ReadOptionChainRequest: Validate: minDistanceBetweenStrikes and minStandardDeviationBetweenStrikes cannot be used together")
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
		if o.EV.StartsAt.IsZero() {
			return fmt.Errorf("ReadOptionChainRequest: Validate: EV.Lookback is required")
		}

		if o.EV.EndsAt.IsZero() {
			return fmt.Errorf("ReadOptionChainRequest: Validate: EV.EndsAt is required")
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

		if signal := query.Get("signal"); signal != "" {
			expectedValue.Signal = signal
		}

		if startsAt := query.Get("starts_at"); startsAt != "" {
			t, err := time.Parse(time.RFC3339, startsAt)
			if err != nil {
				return fmt.Errorf("ReadOptionChainRequest: ParseHTTPRequest: parse starts_at: %w", err)
			}

			expectedValue.StartsAt = t
		}

		if endsAt := query.Get("ends_at"); endsAt != "" {
			t, err := time.Parse(time.RFC3339, endsAt)
			if err != nil {
				return fmt.Errorf("ReadOptionChainRequest: ParseHTTPRequest: parse ends_at: %w", err)
			}

			expectedValue.EndsAt = t
		}

		o.EV = expectedValue
	}

	return nil
}
