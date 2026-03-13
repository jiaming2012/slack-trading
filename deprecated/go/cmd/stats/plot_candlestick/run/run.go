package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ExecPlotCandlestick(projectsDir string, data eventmodels.PlotOrderInputData) (string, error) {
	// Convert the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshalling JSON: %v", err)
	}

	// Prepare the command to run the Python script
	interpreter := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "env", "bin", "python3")
	plotCandlestickPath := path.Join(projectsDir, "slack-trading", "src", "cmd", "stats", "plot_candlestick.py")
	cmd := exec.Command(interpreter, plotCandlestickPath, string(jsonData))

	// Set the standard input to the JSON data
	cmd.Stdin = bytes.NewReader(jsonData)

	// Capture the output
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Run the command
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error running Python script: %v\nOutput: %s", err, out.String())
	}

	// Return the output
	return out.String(), nil
}
