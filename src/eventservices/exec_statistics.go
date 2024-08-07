package eventservices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ExecSignalStatisicalPipelineSpreads(ctx context.Context, projectDir string, lookaheadToOptionContractsMap map[int][]eventmodels.OptionContractV3, stockInfo *eventmodels.StockTickItemDTO, createSignalStatsfunc eventmodels.CreateSignalStatsFunc) (map[string]eventmodels.ExpectedProfitItemSpread, map[string]eventmodels.ExpectedProfitItemSpread, error) {
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

	cmd := exec.Command(interpreter, deriveExpectedProfitPath, "--distributionInDir", distributionInDir, "--json-output", "true", "--shortOnly", "true")
	cmd.Stdin = strings.NewReader(optionsInput)
	cmd.Stderr = os.Stderr

	log.Debugf("running command: %v", cmd.String())

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
			"timestamp": stockInfo.Timestamp,
			"bid":       stockInfo.Bid,
			"ask":       stockInfo.Ask,
		},
	})

	if err != nil {
		return "", fmt.Errorf("getOptionsStandardIn: error marshalling input: %v", err)
	}

	return string(input), nil
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
