package eventservices

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/models"
)

func PlaceTradeSpread(ctx context.Context, url string, bearerToken string, underlying eventmodels.StockSymbol, sellToOpenSymbol eventmodels.OptionSymbol, buyToOpenSymbol eventmodels.OptionSymbol, quantity int, tradeType eventmodels.TradierTradeType, price *float64, tradeDuration eventmodels.TradeDuration, tag string, dryRun bool) error {
	tracer := otel.Tracer("PlaceTradeSpread")
	ctx, span := tracer.Start(ctx, "PlaceTradeSpread", trace.WithAttributes(
		attribute.String("underlying", string(underlying)),
		attribute.String("sellToOpenSymbol", string(sellToOpenSymbol)),
		attribute.String("buyToOpenSymbol", string(buyToOpenSymbol)),
		attribute.Int("quantity", quantity),
		attribute.String("tag", tag),
		attribute.String("tradeType", string(tradeType)),
		attribute.String("tradeDuration", string(tradeDuration)),
	))

	defer span.End()

	logger := log.WithContext(ctx)

	if quantity <= 0 {
		return fmt.Errorf("placeTradeSpread: quantity must be positive")
	}

	quantityStr := strconv.Itoa(quantity)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to create request: %w", err)
	}

	underlyingStr := strings.ToUpper(string(underlying))

	q := req.URL.Query()
	q.Add("class", "multileg")
	q.Add("duration", "GTC")
	q.Add("type", string(tradeType))
	q.Add("duration", string(tradeDuration))
	q.Add("symbol", underlyingStr)
	q.Add("option_symbol[0]", buyToOpenSymbol.NoPrefix())
	q.Add("quantity[0]", quantityStr)
	q.Add("side[0]", "buy_to_open")
	q.Add("option_symbol[1]", sellToOpenSymbol.NoPrefix())
	q.Add("quantity[1]", quantityStr)
	q.Add("side[1]", "sell_to_open")

	if price != nil {
		q.Add("price", fmt.Sprintf("%f", *price))
	}

	if tag != "" {
		q.Add("tag", tag)
	}

	if dryRun {
		q.Add("preview", "true")
	}

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	log.Infof("PlaceTradeSpread: placing trade: %v", req.URL.String())

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to place trade: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("PlaceTradeSpread: failed to place trade, http code %v", res.Status)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to decode response: %w", err)
	}

	if e, found := response["errors"]; found {
		return fmt.Errorf("PlaceTradeSpread: failed to place trade: %v", e)
	}

	logger.Infof("PlaceTradeSpread: placed trade: %v", response)

	return nil
}

func RealizedDrawdown(trade *models.Trade, candles []*models.Candle, state map[string]interface{}) float64 {
	maxDrawdownPrice := 0.0
	if trade.Type == models.TradeTypeBuy {
		for _, t := range candles {
			if trade.Timestamp.After(t.Timestamp) {
				continue
			}

			if maxDrawdownPrice <= 0.0 || t.Low < maxDrawdownPrice {
				maxDrawdownPrice = t.Low
			}
		}
	} else if trade.Type == models.TradeTypeSell {
		for _, t := range candles {
			if trade.Timestamp.After(t.Timestamp) {
				continue
			}

			if maxDrawdownPrice <= 0.0 || t.High > maxDrawdownPrice {
				maxDrawdownPrice = t.High
			}
		}
	}

	return maxDrawdownPrice
}
