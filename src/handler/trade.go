package handler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jiaming2012/slack-trading/src/coingecko"
	"github.com/jiaming2012/slack-trading/src/models"
	"github.com/jiaming2012/slack-trading/src/sheets"
	"github.com/jiaming2012/slack-trading/src/slack"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/message"
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
		Timestamp:     time.Now().UTC(),
		ExecutedPrice: btcPrice,
	}

	for _, param := range params {
		if price, err := parsePrice(param); err == nil {
			trade.RequestedPrice = price
		} else if volume, err := parseVolume(param); err == nil {
			trade.RequestedVolume = volume
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

		p := message.NewPrinter(message.MatchLanguage("en"))
		cmd, responseURL, err := validateForm(r.Form)
		if err != nil {
			log.Error(err)
			return
		}

		if cmd == "/balance" {
			trades, fetchErr := sheets.FetchTrades(ctx)
			if fetchErr != nil {
				errMsg := fmt.Sprintf("Failed to fetch coinbase btc price: %v", fetchErr)
				log.Errorf(errMsg)
				slack.SendResponse(errMsg, responseURL, true)
				return
			}

			btcPrice, fetchErr := coingecko.FetchCoinbaseBTCPrice()
			if fetchErr != nil {
				errMsg := fmt.Sprintf("Failed to fetch coinbase btc price: %v", fetchErr)
				log.Errorf(errMsg)
				slack.SendResponse(errMsg, responseURL, true)
				return
			}

			profit, getStatsErr := trades.GetTradeStats(models.Tick{Bid: btcPrice, Ask: btcPrice})
			if getStatsErr != nil {
				errMsg := fmt.Sprintf("failed to get trade stats: %v", getStatsErr)
				log.Errorf(errMsg)
				slack.SendResponse(errMsg, responseURL, true)
				return
			}
			vwap, volume, _ := trades.GetTradeStatsItems()

			// todo: remove profit.RequestedVolume in favor of volume
			if math.Abs(float64(profit.Volume)-float64(volume)) > 0.001 {
				log.Warnf("Unexpected different volumes: %v, %v", profit.Volume, volume)
			}

			successMsg := p.Sprintf("Open volume: %.2f BTC\nVWAP: %.2f\nMarket: %.2f\nFloating profit: $%.2f\nRealized profit: $%.2f", volume, vwap, btcPrice, profit.FloatingPL, profit.RealizedPL)
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

func TrendSpider(w http.ResponseWriter, r *http.Request) {
	//switch r.Method {
	//case "POST":
	//	fmt.Println("Here we go")
	//	var body interface{}
	//	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
	//		fmt.Printf("err: %v\n", err)
	//		return
	//	}
	//
	//	fmt.Println(bodyz)
	//default:
	//	w.WriteHeader(http.StatusMethodNotAllowed)
	//	log.Errorf("tradeHandler: unsuppored method %s", r.Method)
	//	fmt.Fprintf(w, "traderHandler: unsupported method")
	//}
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
