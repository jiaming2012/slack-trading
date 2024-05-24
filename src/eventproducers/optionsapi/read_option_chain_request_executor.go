package optionsapi

import (
	"net/http"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventservices"
)

type ReadOptionChainRequestExecutor struct {
	OptionsByExpirationURL string
	OptionChainURL         string
	StockURL               string
	BearerToken            string
}

func (s *ReadOptionChainRequestExecutor) Serve(r *http.Request, request eventmodels.ApiRequest3) (chan interface{}, chan error) {
	req := request.(*eventmodels.ReadOptionChainRequest)
	resultCh := make(chan interface{})
	errorCh := make(chan error)

	go func() {
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
	}()

	return resultCh, errorCh
}
