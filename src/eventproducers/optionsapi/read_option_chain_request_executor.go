package optionsapi

import (
	"errors"
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
}

func (s *ReadOptionChainRequestExecutor) serveWithParams(req *eventmodels.ReadOptionChainRequest, resultCh chan interface{}, errorCh chan error) {
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

	// fetch historical candles
	// programPath := "/Users/jamal/projects/slack-trading/src/cmd/stats/import_data/main.go"

	// todo: 15 comes from the EV.Timeframe
	// cmd := exec.Command("go", "run", programPath, "candles-SPX-15", startPeriodStr, endPeriodStr, "est")
	// cmd.Env = append(os.Environ(), fmt.Sprintf("GO_ENV=%s", "development"))
	// cmd.Stderr = os.Stderr

	// if err := cmd.Run(); err != nil {
	// 	log.Fatalf("cmd.Run() failed with %s", err)
	// }

	startPeriodStr := req.EV.StartsAt.Format("2006-01-02T00:00:00")
	endPeriodStr := req.EV.EndsAt.Format("2006-01-02T00:00:00")

	log.Infof("fetching historical candles from startPeriod: %v to endPeriod: %v\n", startPeriodStr, endPeriodStr)

	expectedProfitMap, err := derive_expected_profit.CalculateEV(projectsDir, derive_expected_profit.RunArgs{
		StartsAt:   req.EV.StartsAt,
		EndsAt:     req.EV.EndsAt,
		Ticker:     req.Symbol,
		GoEnv:      "development",
		SignalName: "supertrend_4h_1h_stoch_rsi_15m_up",
	}, options, stockTickItemDTO)

	if err != nil {
		errorCh <- err
		return
	}

	now := time.Now()
	var optionsDTO []*eventmodels.OptionContractV3DTO
	for _, option := range options {
		dto := option.ToDTO(now)

		if expectedProfit, found := expectedProfitMap[option.Description]; found {
			dto.Stats.ExpectedProfit = expectedProfit.ExpectedProfit
			dto.Stats.Premium = expectedProfit.Premium
		}

		optionsDTO = append(optionsDTO, dto)
	}

	sort.Slice(optionsDTO, func(i, j int) bool {
		return optionsDTO[i].Stats.ExpectedProfit > optionsDTO[j].Stats.ExpectedProfit
	})

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
