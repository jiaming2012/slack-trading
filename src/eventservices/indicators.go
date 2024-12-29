package eventservices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func AddIndicatorsToCandles(candles []*eventmodels.PolygonAggregateBarV2, pastCandles []*eventmodels.PolygonAggregateBarV2, indicators []string) ([]*eventmodels.AggregateBarWithIndicators, error) {
	// Get the PROJECTS_DIR environment variable
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return nil, fmt.Errorf("missing PROJECTS_DIR environment variable")
	}

	aggregateCandles := append(pastCandles, candles...)

	// Marshal candles to JSON
	candlesJSON, err := json.Marshal(aggregateCandles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal candles to JSON: %v", err)
	}

	// Trim the spaces from each element in the slice
	for i, indicator := range indicators {
		indicators[i] = strings.TrimSpace(indicator)
	}

	// Run create_indicators.py and pass candles as JSON via standard input
	pythonInterp := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	fileDir := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "create_indicators.py")
	var cmdArgs []string
	if len(indicators) > 0 {
		cmdArgs = append([]string{fileDir, "--indicators"}, indicators...)
	} else {
		cmdArgs = []string{fileDir}
	}
	
	cmd := exec.Command(pythonInterp, cmdArgs...)
	cmd.Stdin = bytes.NewReader(candlesJSON)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run create_indicators.py: %v\n%s", err, out.String())
	}

	// Unmarshall the json output from create_indicators.py
	var data []*eventmodels.AggregateBarWithIndicators
	if err = json.Unmarshal(out.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON output from create_indicators.py: %v", err)
	}

	// Find the index of the first candle in the candles slice
	firstCandleIndex := len(pastCandles)

	return data[firstCandleIndex:], nil
}
