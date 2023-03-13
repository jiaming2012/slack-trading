package handler

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"slack-trading/src/coingecko"
	"slack-trading/src/models"
	"slack-trading/src/slack"
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
		val, err := strconv.ParseFloat(input[1:], 64)
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

	coin, err := coingecko.FetchPrice("bitcoin")
	if err != nil {
		return models.Trade{}, nil
	}

	fmt.Println(coin.Symbol)

	var btcPrice = new(float64)
	for _, ticker := range coin.Tickers {
		if ticker.Base == "BTC" && ticker.Target == "USD" {
			if ticker.Market.Identifier == "gdax" {
				*btcPrice = ticker.LastPrice
				break
			}
		}
	}

	if btcPrice == nil {
		return models.Trade{}, fmt.Errorf("failed to find btc price from coingecko")
	}

	trade := models.Trade{
		Symbol:        "btc",
		Time:          time.Now(),
		ExecutedPrice: *btcPrice,
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

func Trade(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.ParseForm()

		cmd, responseURL, err := validateForm(r.Form)
		if err != nil {
			log.Error(err)
			return
		}

		if cmd == "/add" {
			trade, validationErr := parseBTCRequest(r.Form)
			if validationErr != nil {
				log.Error(validationErr)
				return
			}

			fmt.Println(trade)

			msg := fmt.Sprintf("cool")
			//go defitrader.InsertToken(token)
			go slack.SendResponse(msg, responseURL, true)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "traderHandler: unsupported method")
	}
}
