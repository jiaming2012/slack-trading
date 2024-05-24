package eventmodels

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ReadOptionChainRequest struct {
	Symbol                    StockSymbol  `json:"symbol"`
	OptionTypes               []OptionType `json:"optionTypes"`
	ExpirationsInDays         []int        `json:"expirationsInDays"`
	MinDistanceBetweenStrikes float64      `json:"minDistanceBetweenStrikes"`
	MaxNoOfStrikes            int          `json:"maxNoOfStrikes"`
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

	return nil
}

func (o *ReadOptionChainRequest) ParseHTTPRequest(r *http.Request) error {
	jsonDecoder := json.NewDecoder(r.Body)
	if err := jsonDecoder.Decode(o); err != nil {
		return fmt.Errorf("ReadOptionChainRequest: ParseHTTPRequest: decode: %w", err)
	}

	return nil
}
