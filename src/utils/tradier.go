package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ValidateTag(tag string) error {
	// Maximum lenght of 255 characters.
	// Valid characters are letters, numbers and -
	if len(tag) > 255 {
		return fmt.Errorf("tag is too long: %d", len(tag))
	}

	for _, c := range tag {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
			return fmt.Errorf("invalid character in tag: %c (%s)", c, tag)
		}
	}

	return nil
}

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

func ParseTradierResponse[T any](response []byte) ([]T, error) {
	header := make(map[string]json.RawMessage)

	if err := json.Unmarshal(response, &header); err != nil {
		return nil, fmt.Errorf("ParseTradierResponse(): failed to unmarshal header in response: %w", err)
	}

	var dtos []T

	if len(header) == 1 {
		var key string
		for k := range header {
			key = k
		}

		v := header[key]

		if string(v) == "\"null\"" {
			return []T{}, nil
		}

		data := make(map[string]json.RawMessage)
		if err := json.Unmarshal(v, &data); err != nil {
			return nil, fmt.Errorf("ParseTradierResponse(): failed to unmarshal data in response: %w", err)
		}

		if len(data) == 1 {
			var key string
			for k := range data {
				key = k
			}

			v := data[key]

			var singleDTO T
			if err := json.Unmarshal(v, &singleDTO); err == nil {
				dtos = append(dtos, singleDTO)
			} else {
				if err := json.Unmarshal(v, &dtos); err != nil {
					return nil, fmt.Errorf("ParseTradierResponse(): failed to unmarshal dtos in response: %w", err)
				}
			}
		} else {
			return nil, fmt.Errorf("ParseTradierResponse(): expected 1 key in data, got %v: %v", len(data), data)
		}
	} else {
		return nil, fmt.Errorf("ParseTradierResponse(): expected 1 key in header, got %v: %v", len(header), header)
	}

	return dtos, nil
}
