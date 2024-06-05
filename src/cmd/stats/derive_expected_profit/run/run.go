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

func ExecDeriveExpectedProfit(projectsDir, distributionInDir string, options []eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3) ([]eventmodels.ExpectedProfitItemDTO, error) {
	interpreter := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	deriveExpectedProfitPath := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "derive_expected_profit.py")

	lookahead, err := getLookaheadFromFilePath(distributionInDir)
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error getting lookahead from file path: %v", err)
	}

	filteredOptions, found := lookaheadToOptionContractsMap[lookahead]
	if !found {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: missing options for lookahead: %d", lookahead)
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
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error marshalling input: %v", err)
	}

	cmd := exec.Command(interpreter, deriveExpectedProfitPath, "--distributionInDir", distributionInDir, "--json-output", "true")
	cmd.Stdin = strings.NewReader(string(input))
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error running derive_expected_profit.py: %v", err)
	}

	var results []eventmodels.ExpectedProfitItemDTO
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("ExecDeriveExpectedProfit: error unmarshalling JSON output: %v", err)
	}

	fmt.Printf("results: %v\n", results)

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

	fmt.Println("stderr: ", stderr.String())

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

func FetchEV(projectDir string, args RunArgs, options []eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO) (map[string]eventmodels.ExpectedProfitItemDTO, map[string]eventmodels.ExpectedProfitItemDTO, error) {
	resultMapLong := map[string]eventmodels.ExpectedProfitItemDTO{}
	resultMapShort := map[string]eventmodels.ExpectedProfitItemDTO{}

	switch args.SignalName {
	case "supertrend_4h_1h_stoch_rsi_15m_up":
		lookaheadCandlesCount, lookaheadToOptionContractsMap := calculateLookaheadCandlesCount(time.Now(), options, 15*time.Minute)

		log.Infof("Running supertrend_4h_1h_stoch_rsi_15m_up with lookaheadCandlesCount: %v", lookaheadCandlesCount)

		output, err := supertrend_4h_1h_stoch_rsi_15m_up.Run(supertrend_4h_1h_stoch_rsi_15m_up.RunArgs{
			StartsAt:              args.StartsAt,
			EndsAt:                args.EndsAt,
			Ticker:                args.Ticker,
			LookaheadCandlesCount: lookaheadCandlesCount,
			GoEnv:                 args.GoEnv,
		})

		if err != nil {
			return nil, nil, fmt.Errorf("FetchEV: error running supertrend_4h_1h_stoch_rsi_15m_up: %w", err)
		}

		for _, filePath := range output.ExportedFilepaths {
			log.Infof("fitting distribution for filepath: %s\n", filePath)

			outDir, err := ExecFitDistribution(projectDir, filePath)
			if err != nil {
				return nil, nil, fmt.Errorf("FetchEV: error running fit_distribution.py: %w", err)
			}

			results, err := ExecDeriveExpectedProfit(projectDir, outDir, options, stockInfo, lookaheadToOptionContractsMap)
			if err != nil {
				return nil, nil, fmt.Errorf("FetchEV: error running derive_expected_profit.py: %w", err)
			}

			for _, r := range results {
				if r.DebitPaid != nil {
					resultMapLong[r.Description] = r
				} else if r.CreditReceived != nil {
					resultMapShort[r.Description] = r
				} else {
					return nil, nil, fmt.Errorf("FetchEV: invalid result: %v", r)
				}
			}
		}

	default:
		return nil, nil, fmt.Errorf("FetchEV: unknown signal name: %s", args.SignalName)
	}

	return resultMapLong, resultMapShort, nil
}

func Run(args RunArgs) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	// CalculateEv(args.SignalName, args.Options, args.StockInfo)

	return nil
}
