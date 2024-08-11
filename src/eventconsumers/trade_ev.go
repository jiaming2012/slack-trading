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

func FindHighestEVPerExpiration(options []*eventmodels.OptionSpreadContractDTO) (long []*eventmodels.OptionSpreadContractDTO, short []*eventmodels.OptionSpreadContractDTO, err error) {
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
			highestEVLong = append(highestEVLong, option)
		}
	}

	for _, option := range highestEVShortMap {
		if option.Stats.ExpectedProfitShort > 0 {
			highestEVShort = append(highestEVShort, option)
		}
	}

	return highestEVLong, highestEVShort, nil
}

func SendHighestEVTradeToMarket(ctx context.Context, resultCh chan map[string]interface{}, errCh chan error, event eventmodels.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, goEnv string) error {
	tracer := otel.GetTracerProvider().Tracer("SendHighestEVTradeToMarket")
	ctx, span := tracer.Start(ctx, "SendHighestEVTradeToMarket")
	defer span.End()

	logger := log.WithContext(ctx)

	select {
	case result := <-resultCh:
		if result != nil {
			options, ok := result["options"].(map[string][]*eventmodels.OptionSpreadContractDTO)
			if !ok {
				return fmt.Errorf("options not found in result")
			}

			if calls, ok := options["calls"]; ok {
				highestEVLongCallSpreads, highestEVShortCallSpreads, err := FindHighestEVPerExpiration(calls)
				if err != nil {
					return fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

				for _, spread := range highestEVLongCallSpreads {
					if spread != nil {
						logger.WithField("event", "signal").Infof("Ignoring long call: %v", spread)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Call found")
					}
				}

				for _, spread := range highestEVShortCallSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						if requestedPrc <= 0 {
							return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Put:: requested price must be positive")
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						span.AddEvent("PlaceTradeSpread:Call", trace.WithAttributes(attribute.String("tag", tag)))

						if err := eventservices.PlaceTradeSpread(
							ctx,
							tradierOrderExecuter.Url,
							tradierOrderExecuter.BearerToken,
							event.Symbol,
							spread,
							1,
							eventmodels.TradierTradeTypeCredit,
							&requestedPrc,
							eventmodels.TradeDurationDay,
							tag,
							tradierOrderExecuter.DryRun,
						); err != nil {
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
				highestEVLongPutSpreads, highestEVShortPutSpreads, err := FindHighestEVPerExpiration(puts)
				if err != nil {
					return fmt.Errorf("FindHighestEVPerExpiration: failed to find highest EV per expiration: %w", err)
				}

				for _, spread := range highestEVLongPutSpreads {
					if spread != nil {
						logger.WithField("event", "signal").Infof("Ignoring long put: %v", spread)
					} else {
						logger.WithField("event", "signal").Infof("No Positive EV Long Put found")
					}
				}

				for _, spread := range highestEVShortPutSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						if requestedPrc <= 0 {
							return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Put:: requested price must be positive")
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						span.AddEvent("PlaceTradeSpread:Put", trace.WithAttributes(attribute.String("tag", tag)))


						if err := eventservices.PlaceTradeSpread(
							ctx,
							tradierOrderExecuter.Url,
							tradierOrderExecuter.BearerToken,
							event.Symbol,
							spread,
							1,
							eventmodels.TradierTradeTypeCredit,
							&requestedPrc,
							eventmodels.TradeDurationDay,
							tag,
							tradierOrderExecuter.DryRun,
						); err != nil {
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
