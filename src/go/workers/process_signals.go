package workers

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

func ProcessSignalTriggeredEvent(event models.SignalTriggeredEvent, tradierOrderExecuter *models.TradierOrderExecuter, optionsRequestExecutor *models.ReadOptionChainRequestExecutor, config *models.OptionYAML, loc *time.Location, goEnv string) (*models.ReadOptionChainRequest, error) {
	tracer := otel.GetTracerProvider().Tracer("main:signal")
	ctx, span := tracer.Start(event.Ctx, "<- SignalTriggeredEvent")
	defer span.End()

	logger := log.WithContext(ctx)

	logger.WithField("event", "signal").Infof("tradier executer: %v triggered for %v", event.Signal, event.Symbol)

	telemetry.SignalsGenerated.Add(1)

	startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", config.StartsAt, loc)
	if err != nil {
		return nil, fmt.Errorf("tradier executer: failed to parse startsAt: %v", err)
	}

	endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", config.EndsAt, loc)
	if err != nil {
		return nil, fmt.Errorf("tradier executer: failed to parse endsAt: %v", err)
	}

	span.SetAttributes(attribute.String("symbol", string(event.Symbol)), attribute.String("startsAt", startsAt.String()), attribute.String("endsAt", endsAt.String()))

	return &models.ReadOptionChainRequest{
		Symbol:                    event.Symbol,
		OptionTypes:               []models.OptionType{models.OptionTypeCall, models.OptionTypePut},
		ExpirationsInDays:         config.ExpirationsInDays,
		MinDistanceBetweenStrikes: config.MinDistanceBetweenStrikes,
		MaxNoOfStrikes:            config.MaxNoOfStrikes,
		IsHistorical:              true,
		EV: &models.ReadOptionChainExpectedValue{
			StartsAt: startsAt,
			EndsAt:   endsAt,
			Signal:   event.Signal,
		},
	}, nil
}
