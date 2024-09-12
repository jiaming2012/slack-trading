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
	"github.com/jiaming2012/slack-trading/src/utils"
)

func FindHighestEVPerExpiration(ctx context.Context, options []*eventmodels.OptionSpreadContractDTO, riskProfile *eventmodels.RiskProfileConstraint) (long []*eventmodels.OptionSpreadContractDTO, short []*eventmodels.OptionSpreadContractDTO, err error) {
	tracer := otel.GetTracerProvider().Tracer("getTradeComponents")
	ctx, span := tracer.Start(ctx, "getTradeComponents")
	defer span.End()

	logger := log.WithContext(ctx)

	highestEVLongMap := make(map[time.Time]*eventmodels.OptionSpreadContractDTO)
	highestEVShortMap := make(map[time.Time]*eventmodels.OptionSpreadContractDTO)

	for _, option := range options {
		expiration, err := option.GetExpiration()
		if err != nil {
			err = fmt.Errorf("FindHighestEV: failed to get expiration: %w", err)
			return nil, nil, err
		}

		highestLongEV, found := highestEVLongMap[expiration]
		if found {
			if option.Stats.ExpectedProfitLong > highestLongEV.Stats.ExpectedProfitLong {
				highestEVLongMap[expiration] = option
			}
		} else {
			highestEVLongMap[expiration] = option
		}

		highestShortEV, found := highestEVShortMap[expiration]
		if found {
			if option.Stats.ExpectedProfitShort > highestShortEV.Stats.ExpectedProfitShort {
				highestEVShortMap[expiration] = option
			}
		} else {
			highestEVShortMap[expiration] = option
		}
	}

	var highestEVLong []*eventmodels.OptionSpreadContractDTO
	var highestEVShort []*eventmodels.OptionSpreadContractDTO

	for _, option := range highestEVLongMap {
		if option.Stats.ExpectedProfitLong > 0 {
			if option.DebitPaid == nil {
				return nil, nil, fmt.Errorf("FindHighestEV: DebitPaid is nil")
			}

			risk := *option.DebitPaid * 100.0
			if risk <= 0 {
				return nil, nil, fmt.Errorf("FindHighestEV (long): risk must be positive")
			}

			riskAdjustedExpectedProfit := option.Stats.ExpectedProfitLong / risk
			maxRisk, err := riskProfile.GetMaxRisk(riskAdjustedExpectedProfit)
			if err != nil {
				return nil, nil, fmt.Errorf("FindHighestEV (long): failed to get max risk: %w", err)
			}

			logger.Infof("expected profit long: %f", option.Stats.ExpectedProfitLong)
			logger.Infof("risk: %f", risk)
			logger.Infof("max risk: %f", maxRisk)

			if risk > maxRisk {
				log.Warnf("FindHighestEV (long): risk %f is greater than maxRisk %f", risk, maxRisk)
				continue
			}

			highestEVLong = append(highestEVLong, option)
		}
	}

	for _, option := range highestEVShortMap {
		if option.Stats.ExpectedProfitShort > 0 {
			if option.CreditReceived == nil {
				return nil, nil, fmt.Errorf("FindHighestEV: CreditReceived is nil")
			}

			risk := (math.Abs(option.LongOptionStrikePrice-option.ShortOptionStrikePrice) - *option.CreditReceived) * 100.0
			if risk <= 0 {
				return nil, nil, fmt.Errorf("FindHighestEV (short): risk must be positive")
			}

			riskAdjustedExpectedProfit := option.Stats.ExpectedProfitShort / risk
			maxRisk, err := riskProfile.GetMaxRisk(riskAdjustedExpectedProfit)
			if err != nil {
				return nil, nil, fmt.Errorf("FindHighestEV (short): failed to get max risk: %w", err)
			}

			logger.Infof("expected profit short: %f", option.Stats.ExpectedProfitShort)
			logger.Infof("risk: %f", risk)
			logger.Infof("max risk: %f", maxRisk)

			if risk > maxRisk {
				log.Warnf("FindHighestEV (short): risk %f is greater than maxRisk %f", risk, maxRisk)
				continue
			}

			highestEVShort = append(highestEVShort, option)
		}
	}

	return highestEVLong, highestEVShort, nil
}

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

func getTradeComponents(ctx context.Context, optionType eventmodels.OptionType, options []*eventmodels.OptionSpreadContractDTO, event eventmodels.SignalTriggeredEvent, riskProfileConstraint *eventmodels.RiskProfileConstraint) ([]*eventmodels.TradeSpreadRequestComponents, error) {
	tracer := otel.GetTracerProvider().Tracer("getTradeComponents")
	ctx, span := tracer.Start(ctx, "getTradeComponents")
	defer span.End()

	var side string
	if optionType == eventmodels.OptionTypeCall {
		side = "Call"
	} else if optionType == eventmodels.OptionTypePut {
		side = "Put"
	} else {
		return nil, fmt.Errorf("getTradeComponents: invalid option type: %s", optionType)
	}

	logger := log.WithContext(ctx)

	var results []*eventmodels.TradeSpreadRequestComponents

	highestEVLongSpreads, highestEVShortSpreads, err := FindHighestEVPerExpiration(ctx, options, riskProfileConstraint)
	if err != nil {
		return nil, fmt.Errorf("getTradeComponents: failed to find highest EV per expiration: %w", err)
	}

	for _, spread := range highestEVLongSpreads {
		if spread != nil {
			logger.WithField("event", "signal").Infof("Ignoring long %s: %v", side, spread)
		} else {
			logger.WithField("event", "signal").Infof("No Positive EV Long %s found", side)
		}
	}

	for _, spread := range highestEVShortSpreads {
		if spread != nil {
			requestedPrc := 0.0
			if spread.CreditReceived != nil {
				requestedPrc = *spread.CreditReceived
			}

			if requestedPrc <= 0 {
				logger.Errorf("getTradeComponents: requested price must be positive")
				continue
			}

			tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

			span.AddEvent(fmt.Sprintf("PlaceTradeSpread:%s", side), trace.WithAttributes(attribute.String("tag", tag)))

			results = append(results, &eventmodels.TradeSpreadRequestComponents{
				Tag:            tag,
				Spread:         spread,
				RequestedPrice: requestedPrc,
			})
		} else {
			logger.WithField("event", "signal").Infof("No Positive EV Short %s found", side)
		}
	}

	return results, nil
}

func DeriveHighestEVOrders(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event eventmodels.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, riskProfileConstraint *eventmodels.RiskProfileConstraint) ([]*eventmodels.TradeSpreadRequestComponents, error) {
	tracer := otel.GetTracerProvider().Tracer("DeriveHighestEVOrders")
	ctx, span := tracer.Start(ctx, "DeriveHighestEVOrders")
	defer span.End()

	logger := log.WithContext(ctx)

	var results []*eventmodels.TradeSpreadRequestComponents

	select {
	case result := <-resultCh:
		if result != nil {
			options, ok := result["options"].(map[string][]*eventmodels.OptionSpreadContractDTO)
			if !ok {
				return nil, fmt.Errorf("options not found in result")
			}

			if calls, ok := options["calls"]; ok {
				callResults, err := getTradeComponents(ctx, eventmodels.OptionTypeCall, calls, event, riskProfileConstraint)
				if err == nil {
					results = append(results, callResults...)
				} else {
					logger.Errorf("DeriveHighestEVOrders: failed to get call trade components: %v", err)
				}
			} else {
				logger.Errorf("calls not found in result")
			}

			if puts, ok := options["puts"]; ok {
				putResults, err := getTradeComponents(ctx, eventmodels.OptionTypePut, puts, event, riskProfileConstraint)
				if err == nil {
					results = append(results, putResults...)
				} else {
					return nil, fmt.Errorf("DeriveHighestEVOrders: failed to get put trade components: %w", err)
				}
			} else {
				logger.Errorf("puts not found in result")
			}
		}

	case err := <-errCh:
		return nil, fmt.Errorf("error: %v", err)
	}

	return results, nil
}

func PlaceTradeSpread(ctx context.Context, tradierOrderExecuter *eventmodels.TradierOrderExecuter, tradeRequest eventmodels.PlaceTradeSpreadRequest) error {
	tracer := otel.Tracer("PlaceTradeSpread")
	ctx, span := tracer.Start(ctx, "PlaceTradeSpread", trace.WithAttributes(
		attribute.String("underlying", string(tradeRequest.Underlying)),
		attribute.String("sellToOpenSymbol", string(tradeRequest.Spread.ShortOptionSymbol)),
		attribute.String("buyToOpenSymbol", string(tradeRequest.Spread.LongOptionSymbol)),
		attribute.Int("quantity", tradeRequest.Quantity),
		attribute.String("tag", tradeRequest.Tag),
		attribute.String("tradeType", string(tradeRequest.TradeType)),
		attribute.String("tradeDuration", string(tradeRequest.TradeDuration)),
	))

	defer span.End()

	logger := log.WithContext(ctx)

	if tradeRequest.Quantity <= 0 {
		return fmt.Errorf("placeTradeSpread: quantity must be positive")
	}

	if err := checkMaxNoOfPositions(tradierOrderExecuter, tradeRequest.Underlying, tradeRequest.Quantity, tradeRequest.MaxNoOfPositions); err != nil {
		return fmt.Errorf("placeTradeSpread: failed to check max no of positions: %w", err)
	}

	quantityStr := strconv.Itoa(tradeRequest.Quantity)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, tradierOrderExecuter.Url, nil)
	if err != nil {
		return fmt.Errorf("PlaceTradeSpread: failed to create request: %w", err)
	}

	underlyingStr := strings.ToUpper(string(tradeRequest.Underlying))

	q := req.URL.Query()
	q.Add("class", "multileg")
	q.Add("type", string(tradeRequest.TradeType))
	q.Add("duration", string(tradeRequest.TradeDuration))
	q.Add("symbol", underlyingStr)
	q.Add("option_symbol[0]", tradeRequest.Spread.LongOptionSymbol.NoPrefix())
	q.Add("quantity[0]", quantityStr)
	q.Add("side[0]", "buy_to_open")
	q.Add("option_symbol[1]", tradeRequest.Spread.ShortOptionSymbol.NoPrefix())
	q.Add("quantity[1]", quantityStr)
	q.Add("side[1]", "sell_to_open")
	q.Add("price", fmt.Sprintf("%.2f", tradeRequest.Price))

	if tradeRequest.Tag != "" {
		q.Add("tag", tradeRequest.Tag)
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
