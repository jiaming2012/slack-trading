package optionsapi

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	derive_expected_profit "slack-trading/src/cmd/stats/derive_expected_profit/run"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventservices"
)

type ReadOptionChainRequestExecutor struct {
	OptionsByExpirationURL string
	OptionChainURL         string
	StockURL               string
	BearerToken            string
	GoEnv                  string
}

func (s *ReadOptionChainRequestExecutor) formatOptionContractSpreads(expectedProfitLongSpreadMap map[string]eventmodels.ExpectedProfitItemSpreadDTO, expectedProfitShortSpreadMap map[string]eventmodels.ExpectedProfitItemSpreadDTO) (map[string][]*eventmodels.OptionSpreadContractDTO, error) {
	var callOptionsDTO []*eventmodels.OptionSpreadContractDTO
	var putOptionsDTO []*eventmodels.OptionSpreadContractDTO

	for description, longSpread := range expectedProfitLongSpreadMap {
		shortSpread, found := expectedProfitShortSpreadMap[description]
		if !found {
			return nil, fmt.Errorf("formatOptionContractSpreads: missing short spread for description: %s", description)
		}

		spread := eventmodels.OptionSpreadContractDTO{
			Description:       longSpread.Description,
			DebitPaid:         longSpread.DebitPaid,
			CreditReceived:    shortSpread.CreditReceived,
			LongOptionSymbol:  eventmodels.OptionSymbol(longSpread.LongOptionSymbol),
			ShortOptionSymbol: eventmodels.OptionSymbol(longSpread.ShortOptionSymbol),
		}

		spread.Stats = eventmodels.OptionStats{
			ExpectedProfitLong:  longSpread.ExpectedProfit,
			ExpectedProfitShort: shortSpread.ExpectedProfit,
		}

		if longSpread.Type == eventmodels.OptionTypeCallSpread {
			spread.Type = eventmodels.OptionTypeCallSpread
			callOptionsDTO = append(callOptionsDTO, &spread)
		} else if longSpread.Type == eventmodels.OptionTypePutSpread {
			spread.Type = eventmodels.OptionTypePutSpread
			putOptionsDTO = append(putOptionsDTO, &spread)
		} else {
			return nil, fmt.Errorf("formatOptionContractSpreads: invalid spread type: %s", longSpread.Type)
		}
	}

	sort.Slice(callOptionsDTO, func(i, j int) bool {
		return math.Max(callOptionsDTO[i].Stats.ExpectedProfitLong, callOptionsDTO[i].Stats.ExpectedProfitShort) > math.Max(callOptionsDTO[j].Stats.ExpectedProfitLong, callOptionsDTO[j].Stats.ExpectedProfitShort)
	})

	sort.Slice(putOptionsDTO, func(i, j int) bool {
		return math.Max(putOptionsDTO[i].Stats.ExpectedProfitLong, putOptionsDTO[i].Stats.ExpectedProfitShort) > math.Max(putOptionsDTO[j].Stats.ExpectedProfitLong, putOptionsDTO[j].Stats.ExpectedProfitShort)
	})

	return map[string][]*eventmodels.OptionSpreadContractDTO{
		"calls": callOptionsDTO,
		"puts":  putOptionsDTO,
	}, nil
}

func (s *ReadOptionChainRequestExecutor) formatOptionContracts(options []eventmodels.OptionContractV3, expectedProfitLongMap map[string]eventmodels.ExpectedProfitItemDTO, expectedProfitShortMap map[string]eventmodels.ExpectedProfitItemDTO) []*eventmodels.OptionContractV3DTO {
	now := time.Now()
	var optionsDTO []*eventmodels.OptionContractV3DTO
	for _, option := range options {
		dto := option.ToDTO(now)

		if profitLong, found := expectedProfitLongMap[option.Description]; found {
			if profitLong.DebitPaid == nil {
				continue
			}

			dto.Stats.ExpectedProfitLong = profitLong.ExpectedProfit
		}

		if profitShort, found := expectedProfitShortMap[option.Description]; found {
			if profitShort.CreditReceived == nil {
				continue
			}

			dto.Stats.ExpectedProfitShort = profitShort.ExpectedProfit
		}

		optionsDTO = append(optionsDTO, dto)
	}

	sort.Slice(optionsDTO, func(i, j int) bool {
		return math.Max(optionsDTO[i].Stats.ExpectedProfitLong, optionsDTO[i].Stats.ExpectedProfitShort) > math.Max(optionsDTO[j].Stats.ExpectedProfitLong, optionsDTO[j].Stats.ExpectedProfitShort)
	})

	return optionsDTO
}

func (s *ReadOptionChainRequestExecutor) ServeWithParams(req *eventmodels.ReadOptionChainRequest, bFindSpreads bool, resultCh chan map[string]interface{}, errorCh chan error) {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		errorCh <- errors.New("missing PROJECTS_DIR environment variable")
		return
	}

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

	startPeriodStr := req.EV.StartsAt.Format("2006-01-02T00:00:00")
	endPeriodStr := req.EV.EndsAt.Format("2006-01-02T00:00:00")

	log.Infof("Calculating EV from startPeriod: %v to endPeriod: %v\n", startPeriodStr, endPeriodStr)

	expectedProfitLongMap, expectedProfitShortMap, expectedProfitLongSpreadMap, expectedProfitShortSpreadMap, err := derive_expected_profit.FetchEV(projectsDir, bFindSpreads, derive_expected_profit.RunArgs{
		StartsAt:   req.EV.StartsAt,
		EndsAt:     req.EV.EndsAt,
		Ticker:     req.Symbol,
		GoEnv:      s.GoEnv,
		SignalName: "supertrend_4h_1h_stoch_rsi_15m_up",
	}, options, stockTickItemDTO)

	if err != nil {
		errorCh <- err
		return
	}

	if expectedProfitLongMap != nil && expectedProfitShortMap != nil {
		result["options"] = s.formatOptionContracts(options, expectedProfitLongMap, expectedProfitShortMap)
	} else if expectedProfitLongSpreadMap != nil && expectedProfitShortSpreadMap != nil {
		output, err := s.formatOptionContractSpreads(expectedProfitLongSpreadMap, expectedProfitShortSpreadMap)
		if err != nil {
			errorCh <- err
			return
		}

		result["options"] = output
	}

	resultCh <- result
}

func (s *ReadOptionChainRequestExecutor) serve(req *eventmodels.ReadOptionChainRequest, resultCh chan map[string]interface{}, errorCh chan error) {
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

func (s *ReadOptionChainRequestExecutor) Serve(r *http.Request, request eventmodels.ApiRequest3) (chan map[string]interface{}, chan error) {
	req := request.(*eventmodels.ReadOptionChainRequest)
	resultCh := make(chan map[string]interface{})
	errorCh := make(chan error)

	bFindSpreads := false
	if r.URL.Path == "/options/spreads" {
		bFindSpreads = true
	}

	if req.EV != nil {
		go s.ServeWithParams(req, bFindSpreads, resultCh, errorCh)
	} else {
		go s.serve(req, resultCh, errorCh)
	}

	return resultCh, errorCh
}
