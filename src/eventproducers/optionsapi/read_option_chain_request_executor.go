package optionsapi

import (
	"fmt"
	"net/http"
	"time"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventservices"
)

type ReadOptionChainRequestExecutor struct {
	OptionsByExpirationURL string
	OptionChainURL         string
	StockURL               string
	BearerToken            string
}

func (s *ReadOptionChainRequestExecutor) serveWithParams(req *eventmodels.ReadOptionChainRequest, resultCh chan interface{}, errorCh chan error) {
	options, stockTickItemDTO, err := eventservices.FetchOptionChainWithParamsV3(
		s.OptionsByExpirationURL,
		s.OptionChainURL,
		s.StockURL,
		s.BearerToken,
		req.Symbol,
		req.OptionTypes,
		req.ExpirationsInDays,
		req.MinDistanceBetweenStrikes,
		req.MaxNoOfStrikes,
	)

	if err != nil {
		errorCh <- err
		return
	}

	result := map[string]interface{}{
		"stock": map[string]interface{}{
			"bid": stockTickItemDTO.Bid,
			"ask": stockTickItemDTO.Ask,
		},
	}

	now := time.Now()
	var optionsDTO []*eventmodels.OptionContractV3DTO
	var uniqueExpirationDates = make(map[eventmodels.ExpirationDate]time.Time)
	for _, option := range options {
		optionsDTO = append(optionsDTO, option.ToDTO(now))
		uniqueExpirationDates[option.ExpirationDate] = option.Expiration
	}

	for _, exp := range uniqueExpirationDates {
		fmt.Printf("Expiration: %v\n", exp)

	}

	result["options"] = optionsDTO

	resultCh <- result
}

func (s *ReadOptionChainRequestExecutor) serve(req *eventmodels.ReadOptionChainRequest, resultCh chan interface{}, errorCh chan error) {
	options, stockTickItemDTO, err := eventservices.FetchOptionChainWithParamsV2(
		s.OptionsByExpirationURL,
		s.OptionChainURL,
		s.StockURL,
		s.BearerToken,
		req.Symbol,
		req.OptionTypes,
		req.ExpirationsInDays,
		req.MinDistanceBetweenStrikes,
		req.MaxNoOfStrikes,
	)

	if err != nil {
		errorCh <- err
		return
	}

	result := map[string]interface{}{
		"stock": map[string]interface{}{
			"bid": stockTickItemDTO.Bid,
			"ask": stockTickItemDTO.Ask,
		},
	}

	var optionsDTO []*eventmodels.OptionContractV1DTO
	for _, option := range options {
		optionsDTO = append(optionsDTO, option.ToDTO())
	}

	result["options"] = optionsDTO

	resultCh <- result
}

func (s *ReadOptionChainRequestExecutor) Serve(r *http.Request, request eventmodels.ApiRequest3) (chan interface{}, chan error) {
	req := request.(*eventmodels.ReadOptionChainRequest)
	resultCh := make(chan interface{})
	errorCh := make(chan error)

	if req.EV != nil {
		go s.serveWithParams(req, resultCh, errorCh)
	} else {
		go s.serve(req, resultCh, errorCh)
	}

	return resultCh, errorCh
}
