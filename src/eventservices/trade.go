package eventservices

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

func checkMaxNoOfPositions(tradierOrderExecuter *eventmodels.TradierOrderExecuter, symbol eventmodels.StockSymbol, requestedQty, maxNoOfPositions int) error {
	positionsDTO, err := tradierOrderExecuter.PositionFetcher()
	if err != nil {
		return fmt.Errorf("checkMaxNoOfPositions: failed to fetch positions: %w", err)
	}

	longPositions, shortPositions := 0.0, 0.0
	if requestedQty > 0 {
		longPositions = float64(requestedQty)
	} else if requestedQty < 0 {
		shortPositions = float64(requestedQty)
	}

	for _, dto := range positionsDTO {
		p, err := dto.ToModel()
		if err != nil {
			return fmt.Errorf("checkMaxNoOfPositions: failed to convert dto to model: %w", err)
		}

		option, err := p.Symbol.Components()
		if err != nil {
			return fmt.Errorf("checkMaxNoOfPositions: failed to get underlying: %w", err)
		}

		if eventmodels.NewStockSymbol(option.Underlying) == symbol {
			if p.Quantity > 0 {
				longPositions++
			} else if p.Quantity < 0 {
				shortPositions++
			}
		}
	}

	maxPosition := math.Max(longPositions, shortPositions)
	if maxPosition > float64(maxNoOfPositions) {
		return fmt.Errorf("checkMaxNoOfPositions: max no of positions reached: %v", maxNoOfPositions)
	}

	return nil
}

func PlaceTradeSpread(ctx context.Context, tradierOrderExecuter *eventmodels.TradierOrderExecuter, underlying eventmodels.StockSymbol, spread *eventmodels.OptionSpreadContractDTO, quantity int, tradeType eventmodels.TradierTradeType, price *float64, tradeDuration eventmodels.TradeDuration, tag string, maxNoOfPositions int) error {
	tracer := otel.Tracer("PlaceTradeSpread")
	ctx, span := tracer.Start(ctx, "PlaceTradeSpread", trace.WithAttributes(
		attribute.String("underlying", string(underlying)),
		attribute.String("sellToOpenSymbol", string(spread.ShortOptionSymbol)),
		attribute.String("buyToOpenSymbol", string(spread.LongOptionSymbol)),
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

	if err := checkMaxNoOfPositions(tradierOrderExecuter, underlying, quantity, maxNoOfPositions); err != nil {
		return fmt.Errorf("placeTradeSpread: failed to check max no of positions: %w", err)
	}

	quantityStr := strconv.Itoa(quantity)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, tradierOrderExecuter.Url, nil)
	if err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to create request: %w", err)
	}

	underlyingStr := strings.ToUpper(string(underlying))

	q := req.URL.Query()
	q.Add("class", "multileg")
	q.Add("type", string(tradeType))
	q.Add("duration", string(tradeDuration))
	q.Add("symbol", underlyingStr)
	q.Add("option_symbol[0]", spread.LongOptionSymbol.NoPrefix())
	q.Add("quantity[0]", quantityStr)
	q.Add("side[0]", "buy_to_open")
	q.Add("option_symbol[1]", spread.ShortOptionSymbol.NoPrefix())
	q.Add("quantity[1]", quantityStr)
	q.Add("side[1]", "sell_to_open")

	if price != nil {
		q.Add("price", fmt.Sprintf("%f", *price))
	}

	if tag != "" {
		q.Add("tag", tag)
	}

	if tradierOrderExecuter.DryRun {
		q.Add("preview", "true")
	}

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tradierOrderExecuter.BearerToken))

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
