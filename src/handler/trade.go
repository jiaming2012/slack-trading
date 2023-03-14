package handler

import (
	"context"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"slack-trading/src/coingecko"
	"slack-trading/src/models"
	"slack-trading/src/sheets"
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

func Balance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		ctx := context.Background()

		r.ParseForm()

		cmd, responseURL, err := validateForm(r.Form)
		if err != nil {
			log.Error(err)
			return
		}

		if cmd == "/btc" {
			trades, fetchErr := sheets.FetchTrades(ctx)
			if fetchErr != nil {
				log.Errorf("failed to fetch trades: %v", fetchErr)
				return
			}

			btcPrice, fetchErr := coingecko.FetchCoinbaseBTCPrice()
			if fetchErr != nil {
				slack.SendResponse(fmt.Sprintf("Failed to fetch coinbase btc price: %v", fetchErr), responseURL, true)
				return
			}

			profit := trades.PL(btcPrice)
			successMsg := fmt.Sprintf("Floating profit: %.2f\nRealized profit: %.2f", profit.Floating, profit.Realized)
			slack.SendResponse(successMsg, responseURL, true)
		} else {
			slack.SendResponse(fmt.Sprintf("Unknown cmd: %v", cmd), responseURL, true)
			return
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Errorf("tradeHandler: unsuppored method %s", r.Method)
		fmt.Fprintf(w, "traderHandler: unsupported method")
	}
}

func Trade(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		ctx := context.Background()

		r.ParseForm()

		cmd, responseURL, err := validateForm(r.Form)
		if err != nil {
			log.Error(err)
			return
		}

		if cmd == "/btc" {
			trade, validationErr := parseBTCRequest(r.Form) // todo: refactor parseBTCRequest into service
			if validationErr != nil {
				log.Error(validationErr)
				slack.SendResponse(fmt.Sprintf("Failed to parse BTC request: %v", validationErr), responseURL, true)
				return
			}

			err = sheets.AppendTrade(ctx, &trade)
			if err != nil {
				log.Error(err)
				slack.SendResponse(fmt.Sprintf("Failed to add trade to google sheets: %v", err), responseURL, true)
				return
			}
			////appendRow(ctx, srv, spreadsheetId, "Sheet1")
			////updateRow(ctx, srv, spreadsheetId, "Sheet2")
			////rows, err := fetchRows(ctx, srv, spreadsheetId, "Sheet1", "A3:C7")

			slack.SendResponse(fmt.Sprintf("%v successfully placed", trade), responseURL, false)
		} else {
			slack.SendResponse(fmt.Sprintf("Unknown cmd: %v", cmd), responseURL, true)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Errorf("tradeHandler: unsuppored method %s", r.Method)
		fmt.Fprintf(w, "traderHandler: unsupported method")
	}
}
