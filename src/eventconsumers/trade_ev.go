package eventconsumers

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func FindHighestEVPerExpiration(options []eventmodels.OptionContract) (long []eventmodels.OptionContract, short []eventmodels.OptionContract, err error) {
	highestEVLongMap := make(map[time.Time]eventmodels.OptionContract)
	highestEVShortMap := make(map[time.Time]eventmodels.OptionContract)

	for _, option := range options {
		expiration, err := option.GetExpiration()
		if err != nil {
			err = fmt.Errorf("FindHighestEV: failed to get expiration: %w", err)
			return nil, nil, err
		}

		highestLongEV, found := highestEVLongMap[expiration]
		if found {
			if option.GetLongExpectedProfit() > highestLongEV.GetLongExpectedProfit() {
				highestEVLongMap[expiration] = option
			}
		} else {
			highestEVLongMap[expiration] = option
		}

		highestShortEV, found := highestEVShortMap[expiration]
		if found {
			if option.GetShortExpectedProfit() > highestShortEV.GetShortExpectedProfit() {
				highestEVShortMap[expiration] = option
			}
		} else {
			highestEVShortMap[expiration] = option
		}
	}

	var highestEVLong []eventmodels.OptionContract
	var highestEVShort []eventmodels.OptionContract

	for _, option := range highestEVLongMap {
		if option.GetLongExpectedProfit() > 0 {
			highestEVLong = append(highestEVLong, option)
		}
	}

	for _, option := range highestEVShortMap {
		if option.GetShortExpectedProfit() > 0 {
			highestEVShort = append(highestEVShort, option)
		}
	}

	return highestEVLong, highestEVShort, nil
}

func SendHighestEVTradeToMarket(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, goEnv string) error {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	logger := log.WithContext(ctx)

	select {
	case result := <-resultCh:
		if result != nil {
			options, ok := result["options"].(map[string][]eventmodels.OptionContract)
			if !ok {
				return fmt.Errorf("failed to unmarshal options from result: %v", result)
			}

			if calls, ok := options["calls"]; ok {
				highestEVLongCalls, highestEVShortCalls, err := FindHighestEVPerExpiration(calls)
				if err != nil {
					return fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

				for _, call := range highestEVLongCalls {
					if call != nil {
						logger.WithField("event", "signal").Infof("Ignoring long call: %v", call)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Call found")
					}
				}

				for _, call := range highestEVShortCalls {
					if call != nil {
						requestedPrc := 0.0
						if call.GetCreditReceived() != nil {
							requestedPrc = *call.GetCreditReceived()
						}

						tag := utils.EncodeTag(event.Signal, call.GetShortExpectedProfit(), requestedPrc)

						span.AddEvent("PlaceTradeSpread:Call", trace.WithAttributes(attribute.String("tag", tag)))
						if err := eventservices.PlaceTradeSpread(ctx, tradierOrderExecuter.Url, tradierOrderExecuter.BearerToken, event.Symbol, call.LongOptionSymbol, call.ShortOptionSymbol, 1, tag, tradierOrderExecuter.DryRun); err != nil {
							return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Call:: error placing trade: %v", err)
						}
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Short Call found")
					}
				}
			} else {
				return fmt.Errorf("calls not found in result")
			}

			if puts, ok := options["puts"]; ok {
				highestEVLongPuts, highestEVShortPuts, err := FindHighestEVPerExpiration(puts)
				if err != nil {
					return fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

				for _, put := range highestEVLongPuts {
					if put != nil {
						logger.WithField("event", "signal").Infof("Ignoring long put: %v", put)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Put found")
					}
				}

				for _, put := range highestEVShortPuts {
					if put != nil {
						requestedPrc := 0.0
						if put.GetCreditReceived() != nil {
							requestedPrc = *put.GetCreditReceived()
						}

						tag := utils.EncodeTag(event.Signal, put.GetShortExpectedProfit(), requestedPrc)

						span.AddEvent("PlaceTradeSpread:Put", trace.WithAttributes(attribute.String("tag", tag)))
						if err := eventservices.PlaceTradeSpread(ctx, tradierOrderExecuter.Url, tradierOrderExecuter.BearerToken, event.Symbol, put.LongOptionSymbol, put.ShortOptionSymbol, 1, tag, tradierOrderExecuter.DryRun); err != nil {
							return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Put:: error placing trade: %v", err)
						}
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Short Put found")
					}
				}
			} else {
				return fmt.Errorf("puts not found in result")
			}
		}

	case err := <-errCh:
		return fmt.Errorf("error: %v", err)
	}

	return nil
}
