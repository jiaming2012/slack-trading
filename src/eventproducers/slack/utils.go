package slack

import (
	"errors"
	"fmt"
	"net/url"
	"slack-trading/src/coingecko"
	"slack-trading/src/models"
	"strconv"
	"strings"
	"time"
)

func parsePrice(input string) (float64, error) {
	if input[:1] == "@" {
		val, err := strconv.ParseFloat(input[1:], 64)
		if err != nil {
			return 0, err
		}

		return val, nil
	}

	return 0, errors.New("quantity symbol not found")
}

func parseVolume(input string) (float64, error) {
	if input[:1] == "+" || input[:1] == "-" {
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return 0, err
		}

		return val, nil
	}

	return 0, errors.New("quantity symbol not found")
}

func parseBTCRequest(data url.Values) (models.Trade, error) {
	paramsPayload, ok := data["text"]

	if !ok {
		return models.Trade{}, fmt.Errorf("Could not find text\n")
	}

	if len(paramsPayload) != 1 {
		return models.Trade{}, fmt.Errorf("Invalid paramsPayload length: %d\n", len(paramsPayload))
	}

	params := strings.Fields(paramsPayload[0])

	btcPrice, err := coingecko.FetchCoinbaseBTCPrice()
	if err != nil {
		return models.Trade{}, fmt.Errorf("failed to fetch coinbase btc price: %v", err)
	}

	trade := models.Trade{
		Symbol:        "btc",
		Time:          time.Now(),
		ExecutedPrice: btcPrice,
	}

	for _, param := range params {
		if price, err := parsePrice(param); err == nil {
			trade.RequestedPrice = price
		} else if volume, err := parseVolume(param); err == nil {
			trade.Volume = volume
		} else {
			return models.Trade{}, fmt.Errorf("failed to parse payload param: %v", param)
		}
	}

	return trade, nil
}

func validateForm(data url.Values) (string, string, error) {
	// validate command
	cmd, ok := data["command"]
	if !ok {
		return "", "", fmt.Errorf("could not find command")
	}

	if len(cmd) != 1 {
		return "", "", fmt.Errorf("invalid cmd length: %d", len(cmd))
	}

	// validate response url
	responseURL, ok := data["response_url"]
	if !ok {
		return "", "", fmt.Errorf("could not find response_url")
	}

	if len(responseURL) != 1 {
		return "", "", fmt.Errorf("invalid response_url length: %d\n", len(responseURL))
	}

	return cmd[0], responseURL[0], nil
}
