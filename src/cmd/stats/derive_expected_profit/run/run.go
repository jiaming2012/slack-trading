package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	supertrend_1h_stoch_rsi_15m_down "slack-trading/src/cmd/stats/transform_data/supertrend_1h_stoch_rsi_15m_down/run"
	supertrend_1h_stoch_rsi_15m_up "slack-trading/src/cmd/stats/transform_data/supertrend_1h_stoch_rsi_15m_up/run"
	supertrend_4h_1h_stoch_rsi_15m_down "slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_down/run"
	supertrend_4h_1h_stoch_rsi_15m_up "slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/run"
	"slack-trading/src/eventmodels"
)

type RunArgs struct {
	StartsAt          time.Time
	EndsAt            time.Time
	Ticker            eventmodels.StockSymbol
	GoEnv             string
	SignalName        string
	OptionPricesInDir string
}

func getLookaheadFromFilePath(filePath string) (int, error) {
	// example: /Users/jamal/projects/slack-trading/src/cmd/stats/transform_data/supertrend_4h_1h_stoch_rsi_15m_up/distributions/percent_change-candles-spx-15-from-20240501_093000-to-20240531_160000-lookahead-1215.json
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

func ExecDeriveExpectedProfitSpreads(projectsDir, distributionInDir string, stockInfo *eventmodels.StockTickItemDTO, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3) ([]eventmodels.ExpectedProfitItemSpreadDTO, error) {
	interpreter := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	deriveExpectedProfitPath := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "derive_expected_profit_spreads.py")

	optionsInput, err := getOptionsStandardIn(distributionInDir, stockInfo, lookaheadToOptionContractsMap)
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfitSpreads: error getting options standard input: %v", err)
	}

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

func ExecFitDistribution(projectsDir, percentChangeInDir string) (string, error) {
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

func ExecSignalStatisicalPipelineSpreads(projectDir string, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, createSignalStatsfunc CreateSignalStatsFunc) (map[string]eventmodels.ExpectedProfitItemSpread, map[string]eventmodels.ExpectedProfitItemSpread, error) {
	output, err := createSignalStatsfunc()

	if err != nil {
		return nil, nil, fmt.Errorf("FetchEV: error running supertrend_4h_1h_stoch_rsi_15m_down: %w", err)
	}

	resultMapLongSpread := make(map[string]eventmodels.ExpectedProfitItemSpread)
	resultMapShortSpread := make(map[string]eventmodels.ExpectedProfitItemSpread)

	for _, filePath := range output.ExportedFilepaths {
		log.Infof("fitting distribution for filepath: %s", filePath)

		outDir, err := ExecFitDistribution(projectDir, filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("FetchEV: error running fit_distribution.py: %w", err)
		}

		resultsDTO, err := ExecDeriveExpectedProfitSpreads(projectDir, outDir, stockInfo, lookaheadToOptionContractsMap)
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

func FetchEVSpreads(projectDir string, bFindSpreads bool, args RunArgs, options []eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO) (map[string]eventmodels.ExpectedProfitItemSpread, map[string]eventmodels.ExpectedProfitItemSpread, error) {
	lookaheadCandlesCount, lookaheadToOptionContractsMap := calculateLookaheadCandlesCount(time.Now(), options, 15*time.Minute)

	log.Infof("FetchEVSpreads: fetching EV for signal: %s", args.SignalName)

	switch args.SignalName {
	case "supertrend_1h_stoch_rsi_15m_up":
		log.Infof("Running supertrend_1h_stoch_rsi_15m_up with lookaheadCandlesCount: %v", lookaheadCandlesCount)

		return ExecSignalStatisicalPipelineSpreads(projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_1h_stoch_rsi_15m_up.Run(supertrend_1h_stoch_rsi_15m_up.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	case "supertrend_1h_stoch_rsi_15m_down":
		log.Infof("Running supertrend_1h_stoch_rsi_15m_down with lookaheadCandlesCount: %v", lookaheadCandlesCount)

		return ExecSignalStatisicalPipelineSpreads(projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_1h_stoch_rsi_15m_down.Run(supertrend_1h_stoch_rsi_15m_down.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	case "supertrend_4h_1h_stoch_rsi_15m_down":
		log.Infof("Running supertrend_4h_1h_stoch_rsi_15m_down with lookaheadCandlesCount: %v", lookaheadCandlesCount)

		return ExecSignalStatisicalPipelineSpreads(projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
			return supertrend_4h_1h_stoch_rsi_15m_down.Run(supertrend_4h_1h_stoch_rsi_15m_down.RunArgs{
				StartsAt:              args.StartsAt,
				EndsAt:                args.EndsAt,
				Ticker:                args.Ticker,
				LookaheadCandlesCount: lookaheadCandlesCount,
				GoEnv:                 args.GoEnv,
			})
		})

	case "supertrend_4h_1h_stoch_rsi_15m_up":
		log.Infof("Running supertrend_4h_1h_stoch_rsi_15m_up with lookaheadCandlesCount: %v", lookaheadCandlesCount)

		return ExecSignalStatisicalPipelineSpreads(projectDir, lookaheadToOptionContractsMap, stockInfo, func() (eventmodels.SignalRunOutput, error) {
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
