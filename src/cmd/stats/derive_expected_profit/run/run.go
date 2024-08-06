package run

import (
	"fmt"
	"os"
	"time"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type RunArgs struct {
	StartsAt          time.Time
	EndsAt            time.Time
	Ticker            eventmodels.StockSymbol
	GoEnv             string
	SignalName        eventmodels.SignalName
	OptionPricesInDir string
}

func ExecSignalStatisicalPipeline(projectDir string, options []eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, args RunArgs) ([]eventmodels.ExpectedProfitItem, []eventmodels.ExpectedProfitItem, error) {
	// 	resultsDTO, err := ExecDeriveExpectedProfit(projectDir, outDir, stockInfo, lookaheadToOptionContractsMap)
	// 	if err != nil {
	// 		return nil, nil, nil, nil, fmt.Errorf("FetchEV: error running derive_expected_profit.py: %w", err)
	// 	}

	// 	for _, dto := range resultsDTO {
	// 		r, err := dto.ToModel()
	// 		if err != nil {
	// 			return nil, nil, nil, nil, fmt.Errorf("FetchEV: ExecDeriveExpectedProfit: error converting results to model: %w", err)
	// 		}

	// 		if r.DebitPaid != nil {
	// 			resultMapLong[r.Description] = *r
	// 		} else if r.CreditReceived != nil {
	// 			resultMapShort[r.Description] = *r
	// 		} else {
	// 			return nil, nil, nil, nil, fmt.Errorf("FetchEV: invalid result: %v", r)
	// 		}
	// 	}
	return nil, nil, fmt.Errorf("ExecSignalStatisicalPipeline: not implemented")
}


type CreateSignalStats interface {
	Run() (eventmodels.SignalRunOutput, error)
}

func Run(args RunArgs) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	// CalculateEv(args.SignalName, args.Options, args.StockInfo)

	return nil
}
