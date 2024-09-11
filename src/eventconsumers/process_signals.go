package eventconsumers

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func ProcessSignalTriggeredEvent(event eventmodels.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, optionsRequestExecutor *eventmodels.ReadOptionChainRequestExecutor, config *eventmodels.OptionYAML, loc *time.Location, goEnv string) (*eventmodels.ReadOptionChainRequest, error) {
	tracer := otel.GetTracerProvider().Tracer("main:signal")
	ctx, span := tracer.Start(event.Ctx, "<- SignalTriggeredEvent")
	defer span.End()

	logger := log.WithContext(ctx)

	logger.WithField("event", "signal").Infof("tradier executer: %v triggered for %v", event.Signal, event.Symbol)

	startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", config.StartsAt, loc)
	if err != nil {
		return nil, fmt.Errorf("tradier executer: failed to parse startsAt: %v", err)
	}

	endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", config.EndsAt, loc)
	if err != nil {
		return nil, fmt.Errorf("tradier executer: failed to parse endsAt: %v", err)
	}

	span.SetAttributes(attribute.String("symbol", string(event.Symbol)), attribute.String("startsAt", startsAt.String()), attribute.String("endsAt", endsAt.String()))

	return &eventmodels.ReadOptionChainRequest{
		Symbol:                    event.Symbol,
		OptionTypes:               []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut},
		ExpirationsInDays:         config.ExpirationsInDays,
		MinDistanceBetweenStrikes: config.MinDistanceBetweenStrikes,
		MaxNoOfStrikes:            config.MaxNoOfStrikes,
		IsHistorical:              true,
		EV: &eventmodels.ReadOptionChainExpectedValue{
			StartsAt: startsAt,
			EndsAt:   endsAt,
			Signal:   event.Signal,
		},
	}, nil
}
