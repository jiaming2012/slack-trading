package run

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
)

func PlaceTradeSpread(ctx context.Context, url string, bearerToken string, underlying eventmodels.StockSymbol, sellToOpenSymbol eventmodels.OptionSymbol, buyToOpenSymbol eventmodels.OptionSymbol, quantity int, tag string, dryRun bool) error {
	tracer := otel.Tracer("PlaceTradeSpread")
	ctx, span := tracer.Start(ctx, "PlaceTradeSpread", trace.WithAttributes(
		attribute.String("underlying", string(underlying)),
		attribute.String("sellToOpenSymbol", string(sellToOpenSymbol)),
		attribute.String("buyToOpenSymbol", string(buyToOpenSymbol)),
		attribute.Int("quantity", quantity),
		attribute.String("tag", tag),
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
	q.Add("type", "market")
	q.Add("symbol", underlyingStr)
	q.Add("option_symbol[0]", string(buyToOpenSymbol))
	q.Add("quantity[0]", quantityStr)
	q.Add("side[0]", "buy_to_open")
	q.Add("option_symbol[1]", string(sellToOpenSymbol))
	q.Add("quantity[1]", quantityStr)
	q.Add("side[1]", "sell_to_open")

	if tag != "" {
		q.Add("tag", tag)
	}

	if dryRun {
		q.Add("preview", "true")
	}

	req.URL.RawQuery = q.Encode()
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

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
