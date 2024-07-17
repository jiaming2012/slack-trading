package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	supertrend_1h_stoch_rsi_15m_down "github.com/jiaming2012/slack-trading/src/cmd/stats/transform_data/supertrend_1h_stoch_rsi_15m_down/run"
	supertrend_1h_stoch_rsi_15m_up "github.com/jiaming2012/slack-trading/src/cmd/stats/transform_data/supertrend_1h_stoch_rsi_15m_up/run"
	supertrend_4h_1h_stoch_rsi_15m_down "github.com/jiaming2012/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_down/run"
	supertrend_4h_1h_stoch_rsi_15m_up "github.com/jiaming2012/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/run"
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

func getLookaheadFromFilePath(filePath string) (int, error) {
	// example: /Users/jamal/projects/github.com/jiaming2012/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/distributions/percent_change-candles-spx-15-from-20240501_093000-to-20240531_160000-lookahead-1215.json
	// lookahead-1215.json
	lookaheadStr := strings.Split(filePath, "-lookahead-")[1]
	lookaheadStr = strings.Split(lookaheadStr, ".")[0]

	lookahead, err := strconv.Atoi(lookaheadStr)
	if err != nil {
		return 0, fmt.Errorf("getLookaheadFromFilePath: error converting lookahead to int: %v", err)
	}

	return lookahead, nil
}

func getOptionsStandardIn(distributionInDir string, stockInfo *eventmodels.StockTickItemDTO, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3) (string, error) {
	lookahead, err := getLookaheadFromFilePath(distributionInDir)
	if err != nil {
		return "", fmt.Errorf("getOptionsStandardIn: error getting lookahead from file path: %v", err)
	}

	filteredOptions, found := lookaheadToOptionContractsMap[lookahead]
	if !found {
		return "", fmt.Errorf("getOptionsStandardIn: missing options for lookahead: %d", lookahead)
	}

	var filteredOptionsDTO []*eventmodels.OptionContractV1DTO
	for _, option := range filteredOptions {
		filteredOptionsDTO = append(filteredOptionsDTO, option.ToDTOV1())
	}

	input, err := json.Marshal(map[string]interface{}{
		"options": filteredOptionsDTO,
		"stock": map[string]interface{}{
			"bid": stockInfo.Bid,
			"ask": stockInfo.Ask,
		},
	})

	if err != nil {
		return "", fmt.Errorf("getOptionsStandardIn: error marshalling input: %v", err)
	}

	return string(input), nil
}

func ExecDeriveExpectedProfitSpreads(ctx context.Context, projectsDir, distributionInDir string, stockInfo *eventmodels.StockTickItemDTO, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3) ([]eventmodels.ExpectedProfitItemSpreadDTO, error) {
	tracer := otel.Tracer("ExecDeriveExpectedProfitSpreads")
	_, span := tracer.Start(ctx, "ExecDeriveExpectedProfitSpreads")
	defer span.End()

	var keys []int64
	for k := range lookaheadToOptionContractsMap {
		keys = append(keys, int64(k))
	}

	span.SetAttributes(attribute.String("distributionInDir", distributionInDir), attribute.Int64Slice("lookaheadToOptionContractsMapKeys", keys))

	interpreter := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	deriveExpectedProfitPath := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "derive_expected_profit_spreads.py")

	optionsInput, err := getOptionsStandardIn(distributionInDir, stockInfo, lookaheadToOptionContractsMap)
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfitSpreads: error getting options standard input: %v", err)
	}

	// write optionsInput to file
	// optionsInputPath := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "options_input.json")
	// if err := ioutil.WriteFile(optionsInputPath, []byte(optionsInput), 0644); err != nil {
	// 	return nil, fmt.Errorf("ExecDeriveExpectedProfitSpreads: error writing options input to file: %v", err)
	// }

	cmd := exec.Command(interpreter, deriveExpectedProfitPath, "--distributionInDir", distributionInDir, "--json-output", "true")
	cmd.Stdin = strings.NewReader(optionsInput)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfitSpreads: error running derive_expected_profit.py: %v", err)
	}

	var results []eventmodels.ExpectedProfitItemSpreadDTO
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfitSpreads: error unmarshalling JSON output: %v", err)
	}

	return results, nil
}

func ExecDeriveExpectedProfit(projectsDir, distributionInDir string, stockInfo *eventmodels.StockTickItemDTO, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3) ([]eventmodels.ExpectedProfitItemDTO, error) {
	interpreter := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	deriveExpectedProfitPath := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "derive_expected_profit.py")

	optionsInput, err := getOptionsStandardIn(distributionInDir, stockInfo, lookaheadToOptionContractsMap)
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error getting options standard input: %v", err)
	}

	cmd := exec.Command(interpreter, deriveExpectedProfitPath, "--distributionInDir", distributionInDir, "--json-output", "true")
	cmd.Stdin = strings.NewReader(optionsInput)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error running derive_expected_profit.py: %v", err)
	}

	var results []eventmodels.ExpectedProfitItemDTO
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error unmarshalling JSON output: %v", err)
	}

	return results, nil
}

func ExecFitDistribution(ctx context.Context, projectsDir, percentChangeInDir string) (string, error) {
	tracer := otel.Tracer("ExecFitDistribution")
	_, span := tracer.Start(ctx, "ExecFitDistribution", trace.WithAttributes(attribute.String("percentChangeInDir", percentChangeInDir)))
	defer span.End()

	interpreter := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	fitDistributionPath := fmt.Sprintf("%s/fit_distribution.py", path.Join(projectsDir, "slack-trading", "src", "cmd", "stats"))

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(interpreter, fitDistributionPath, "--inDir", percentChangeInDir, "--json-output", "true")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ExecFitDistribution: error running fit_distribution.py: %v, stderr: %s", err, stderr.String())
	}

	output := stdout.Bytes()

	var results map[string]interface{}
	if err := json.Unmarshal(output, &results); err != nil {
		return "", fmt.Errorf("ExecFitDistribution: error unmarshalling JSON output: %v", err)
	}

	if outDir, found := results["outDir"]; found {
		return outDir.(string), nil
	}

	return "", fmt.Errorf("ExecFitDistribution: missing outDir in JSON output")
}

func calculateLookaheadCandlesCount(now time.Time, options []eventmodels.OptionContractV3, candleDuration time.Duration) ([]int, map[int][]eventmodels.OptionContractV3) {
	var uniqueExpirationDates = make(map[eventmodels.ExpirationDate]eventmodels.OptionContractV3)
	lookaheadToOptionContractsMap := make(map[int][]eventmodels.OptionContractV3)

	for _, option := range options {
		uniqueExpirationDates[option.ExpirationDate] = option
	}

	lookaheadCandlesCount := []int{}
	optionExpirationToLookahead := make(map[eventmodels.ExpirationDate]int)
	for _, option := range uniqueExpirationDates {
		timeToExpiration := option.TimeUntilExpiration(now)
		if timeToExpiration.Minutes() > 0 {
			l := int(timeToExpiration.Minutes() / candleDuration.Minutes())
			lookaheadCandlesCount = append(lookaheadCandlesCount, l)
			optionExpirationToLookahead[option.ExpirationDate] = l
		}
	}

	for _, option := range options {
		if l, found := optionExpirationToLookahead[option.ExpirationDate]; found {
			lookaheadToOptionContractsMap[l] = append(lookaheadToOptionContractsMap[l], option)
		}
	}

	return lookaheadCandlesCount, lookaheadToOptionContractsMap
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

type CreateSignalStatsFunc func() (eventmodels.SignalRunOutput, error)
type CreateSignalStats interface {
	Run() (eventmodels.SignalRunOutput, error)
}

func ExecSignalStatisicalPipelineSpreads(ctx context.Context, projectDir string, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, createSignalStatsfunc CreateSignalStatsFunc) (map[string]eventmodels.ExpectedProfitItemSpread, map[string]eventmodels.ExpectedProfitItemSpread, error) {
	tracer := otel.Tracer("ExecSignalStatisicalPipelineSpreads")
	_, span := tracer.Start(ctx, "ExecSignalStatisicalPipelineSpreads", trace.WithAttributes(attribute.String("symbol", string(stockInfo.Symbol))))
	defer span.End()

	logger := log.WithContext(ctx)

	span.AddEvent("Executing signal stats ...")
	output, err := createSignalStatsfunc()
	span.AddEvent("Executed signal stats")

	if err != nil {
		return nil, nil, fmt.Errorf("FetchEV: error running supertrend_4h_1h_stoch_rsi_15m_down: %w", err)
	}

	resultMapLongSpread := make(map[string]eventmodels.ExpectedProfitItemSpread)
	resultMapShortSpread := make(map[string]eventmodels.ExpectedProfitItemSpread)

	logger.Infof("exported %v files", len(output.ExportedFilepaths))

	for _, filePath := range output.ExportedFilepaths {
		logger.Infof("fitting distribution for filepath: %s", filePath)

		outDir, err := ExecFitDistribution(ctx, projectDir, filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("FetchEV: error running fit_distribution.py: %w", err)
		}

		resultsDTO, err := ExecDeriveExpectedProfitSpreads(ctx, projectDir, outDir, stockInfo, lookaheadToOptionContractsMap)
		if err != nil {
			return nil, nil, fmt.Errorf("FetchEV: error running derive_expected_profit_spreads.py: %w", err)
		}

		var results []eventmodels.ExpectedProfitItemSpread
		for _, dto := range resultsDTO {
			r, err := dto.ToModel()
			if err != nil {
				return nil, nil, fmt.Errorf("FetchEV: error converting results to model: %w", err)
			}

			results = append(results, *r)
		}

		for _, r := range results {
			if r.DebitPaid != nil {
				resultMapLongSpread[r.Description] = r
			} else if r.CreditReceived != nil {
				resultMapShortSpread[r.Description] = r
			} else {
				return nil, nil, fmt.Errorf("FetchEV: invalid result: %v", r)
			}
		}
	}

	return resultMapLongSpread, resultMapShortSpread, nil
}

func FetchEVSpreads(ctx context.Context, projectDir string, bFindSpreads bool, args RunArgs, options []eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, now time.Time) (map[string]eventmodels.ExpectedProfitItemSpread, map[string]eventmodels.ExpectedProfitItemSpread, error) {
	tracer := otel.Tracer("FetchEVSpreads")
	_, span := tracer.Start(ctx, "FetchEVSpreads")
	defer span.End()

	logger := log.WithContext(ctx)

	lookaheadCandlesCount, lookaheadToOptionContractsMap := calculateLookaheadCandlesCount(now, options, 15*time.Minute)

	logger.Infof("Running %v with lookaheadCandlesCount: %v", args.SignalName, lookaheadCandlesCount)

	switch args.SignalName {
	case eventmodels.SuperTrend1hStochRsi15mUp:
		span.AddEvent("Executing SuperTrend1hStochRsi15mUp")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_1h_stoch_rsi_15m_up.Run(supertrend_1h_stoch_rsi_15m_up.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	case eventmodels.SuperTrend1hStochRsi15mDown:
		span.AddEvent("Executing SuperTrend1hStochRsi15mDown")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_1h_stoch_rsi_15m_down.Run(supertrend_1h_stoch_rsi_15m_down.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	case eventmodels.SuperTrend4h1hStochRsi15mDown:
		span.AddEvent("Executing SuperTrend4h1hStochRsi15mDown")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_4h_1h_stoch_rsi_15m_down.Run(supertrend_4h_1h_stoch_rsi_15m_down.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	case eventmodels.SuperTrend4h1hStochRsi15mUp:
		span.AddEvent("Executing SuperTrend4h1hStochRsi15mUp")
		return ExecSignalStatisicalPipelineSpreads(ctx, projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_4h_1h_stoch_rsi_15m_up.Run(supertrend_4h_1h_stoch_rsi_15m_up.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	default:
		return nil, nil, fmt.Errorf("FetchEV: unknown signal name: %s", args.SignalName)
	}
}

func Run(args RunArgs) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	// CalculateEv(args.SignalName, args.Options, args.StockInfo)

	return nil
}
