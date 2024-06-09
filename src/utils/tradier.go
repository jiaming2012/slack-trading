package utils

import (
	"fmt"
	"strings"

	"slack-trading/src/eventmodels"
)

func EncodeTag(signal eventmodels.SignalName, expectedProfit float64, requestedPrc float64) string {
	signal_part := strings.Replace(string(signal), "-", "--", -1)
	signal_part = strings.Replace(signal_part, "_", "-", -1)
	expectedProfit_part := strings.Replace(fmt.Sprintf("%.2f", expectedProfit), ".", "-", -1)
	requestedPrc_part := strings.Replace(fmt.Sprintf("%.2f", requestedPrc), ".", "-", -1)

	return fmt.Sprintf("%s---%s---%s", signal_part, expectedProfit_part, requestedPrc_part)
}

// supertrend-4h-1h_stoch_rsi_15m_up
// "supertrend--4h--1h-stoch-rsi-15m-up---9-53---21-45"
func DecodeTag(tag string) (eventmodels.SignalName, float64, float64, error) {
	parts := strings.Split(tag, "---")
	if len(parts) != 3 {
		return "", 0, 0, fmt.Errorf("invalid tag: expected 3 parts: %s", tag)
	}

	signal_part := strings.Replace(parts[0], "--", ".", -1)
	signal_part = strings.Replace(signal_part, "-", "_", -1)
	signal_part = strings.Replace(signal_part, ".", "-", -1)
	expectedProfit_part := strings.Replace(parts[1], "-", ".", -1)
	requestedPrc_part := strings.Replace(parts[2], "-", ".", -1)

	signal := eventmodels.SignalName(signal_part)
	expectedProfit := 0.0
	requestedPrc := 0.0

	if _, err := fmt.Sscanf(expectedProfit_part, "%f", &expectedProfit); err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse expectedProfit: %w", err)
	}

	if _, err := fmt.Sscanf(requestedPrc_part, "%f", &requestedPrc); err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse requestedPrc: %w", err)
	}

	return signal, expectedProfit, requestedPrc, nil
}
