package slack

import (
	"errors"
	"fmt"
	"github.com/gorilla/schema"
	"math"
	"net/url"
	"slack-trading/src/eventmodels"
	models "slack-trading/src/eventmodels"
	"strconv"
	"strings"
	"time"
)

var NoRequestParamsErr = fmt.Errorf("no request params found")

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

func parseBalanceRequest(data url.Values) (string, error) {
	return "btc", nil
}

func parseAccountRequestParams(params string) (interface{}, error) {
	if len(params) == 0 {
		return nil, NoRequestParamsErr
	}

	tokens := strings.Split(params, " ")
	if len(tokens) == 0 {
		return nil, fmt.Errorf("failed to split params: %v", params)
	}

	switch tokens[0] {
	case "add":
		if len(tokens) < 5 {
			return nil, fmt.Errorf("add account command must have at least 5 tokens. Found %v", len(tokens))
		}

		if math.Mod(float64(len(tokens)-5), 3) != 0 {
			return nil, fmt.Errorf("each price level must have 3 components. ExecutedPrice Levels: %v", tokens[3:])
		}

		balance, err := strconv.ParseFloat(tokens[2], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse account balance: %v. error=%v", tokens[2], err)
		}

		maxLossPercentage, err := strconv.ParseFloat(tokens[3], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse max loss percentage: %v. error=%v", tokens[3], err)
		}

		priceLevels := make([][3]float64, 0)
		for i := 4; i < len(tokens)-1; i += 3 {
			param1, err := strconv.ParseFloat(tokens[i], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse priceLevel(param1): %v. error=%v", tokens[i], err)
			}

			param2, err := strconv.ParseFloat(tokens[i+1], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse priceLevel(param2): %v. error=%v", tokens[i+1], err)
			}

			param3, err := strconv.ParseFloat(tokens[i+2], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse priceLevel(param3): %v. error=%v", tokens[i+2], err)
			}

			priceLevels = append(priceLevels, [3]float64{param1, param2, param3})
		}

		finalPriceLevel, err := strconv.ParseFloat(tokens[len(tokens)-1], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse final price level: %v. error=%v", tokens[len(tokens)-1], err)
		}

		priceLevels = append(priceLevels, [3]float64{finalPriceLevel, 0, 0})

		return models.AddAccountRequestEvent{
			Name:              tokens[1],
			Balance:           balance,
			MaxLossPercentage: maxLossPercentage,
			PriceLevelsInput:  priceLevels,
		}, nil
	default:
		return nil, fmt.Errorf("parseAccountRequestParams: unidentified token %v", tokens[0])
	}
}

func parseAccountRequest(data url.Values) (interface{}, error) {
	req := new(models.IncomingSlackRequest)
	schema.NewDecoder().Decode(req, data)

	request, err := parseAccountRequestParams(req.Params)
	if err == NoRequestParamsErr {
		return eventmodels.GetAccountsRequestEvent{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("parseAccountRequestParams failed: %v", err)
	}

	return request, nil
}

func parseBTCTradeRequest(data url.Values) (eventmodels.TradeRequestEvent, error) {
	paramsPayload, ok := data["text"]

	if !ok {
		return eventmodels.TradeRequestEvent{}, fmt.Errorf("Could not find text\n")
	}

	if len(paramsPayload) != 1 {
		return eventmodels.TradeRequestEvent{}, fmt.Errorf("Invalid paramsPayload length: %d\n", len(paramsPayload))
	}

	params := strings.Fields(paramsPayload[0])

	tradeReq := eventmodels.TradeRequestEvent{
		Timestamp: time.Now().UTC(),
		Symbol:    "btc",
	}

	for _, param := range params {
		if price, err := parsePrice(param); err == nil {
			tradeReq.Price = price
		} else if volume, err := parseVolume(param); err == nil {
			tradeReq.Volume = volume
		} else {
			return eventmodels.TradeRequestEvent{}, fmt.Errorf("failed to parse payload param: %v", param)
		}
	}

	return tradeReq, nil
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
