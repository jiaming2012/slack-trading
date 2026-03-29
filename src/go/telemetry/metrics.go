package telemetry

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metric instruments -- nil until Init() is called.
var (
	OrdersPlaced      metric.Int64Counter
	OrdersFilled      metric.Int64Counter
	OrdersRejected    metric.Int64Counter
	CandlesProcessed  metric.Int64Counter
	SignalsGenerated  metric.Int64Counter
	ActivePlaygrounds metric.Int64Gauge
	OpenOrders        metric.Int64Gauge
	UptimeSeconds     metric.Float64Gauge
)

// Init creates all metric instruments using the global MeterProvider.
// MUST be called after utils.SetupOTelSDK() so the real provider is available.
func Init() error {
	meter := otel.GetMeterProvider().Meter("grodt")

	var err error

	OrdersPlaced, err = meter.Int64Counter("grodt.orders.placed",
		metric.WithUnit("{order}"),
		metric.WithDescription("Number of orders placed"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create OrdersPlaced counter: %w", err)
	}

	OrdersFilled, err = meter.Int64Counter("grodt.orders.filled",
		metric.WithUnit("{order}"),
		metric.WithDescription("Number of orders filled"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create OrdersFilled counter: %w", err)
	}

	OrdersRejected, err = meter.Int64Counter("grodt.orders.rejected",
		metric.WithUnit("{order}"),
		metric.WithDescription("Number of orders rejected"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create OrdersRejected counter: %w", err)
	}

	CandlesProcessed, err = meter.Int64Counter("grodt.candles.processed",
		metric.WithUnit("{candle}"),
		metric.WithDescription("Number of candles processed"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create CandlesProcessed counter: %w", err)
	}

	SignalsGenerated, err = meter.Int64Counter("grodt.signals.generated",
		metric.WithUnit("{signal}"),
		metric.WithDescription("Number of signals generated"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create SignalsGenerated counter: %w", err)
	}

	ActivePlaygrounds, err = meter.Int64Gauge("grodt.heartbeat.active_playgrounds",
		metric.WithUnit("{playground}"),
		metric.WithDescription("Number of active playgrounds"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create ActivePlaygrounds gauge: %w", err)
	}

	OpenOrders, err = meter.Int64Gauge("grodt.heartbeat.open_orders",
		metric.WithUnit("{order}"),
		metric.WithDescription("Number of open orders"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create OpenOrders gauge: %w", err)
	}

	UptimeSeconds, err = meter.Float64Gauge("grodt.heartbeat.uptime",
		metric.WithUnit("s"),
		metric.WithDescription("Server uptime in seconds"))
	if err != nil {
		return fmt.Errorf("telemetry.Init: failed to create UptimeSeconds gauge: %w", err)
	}

	return nil
}

// ShouldEmitOrderTelemetry returns true if order telemetry should be emitted
// for the given playground environment. Only live and reconcile playgrounds
// produce telemetry; simulator playgrounds are excluded.
func ShouldEmitOrderTelemetry(env string) bool {
	return env == "live" || env == "reconcile"
}

// ClientIDOrEmpty safely dereferences a *string to a string for client_id.
// Returns empty string if the pointer is nil.
func ClientIDOrEmpty(clientID *string) string {
	if clientID != nil {
		return *clientID
	}
	return ""
}

// PlaygroundAttrs returns OTel metric attributes for a playground,
// including environment, account_type, and client_id dimensions.
func PlaygroundAttrs(env string, accountType string, clientID string) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("environment", env),
		attribute.String("account_type", accountType),
		attribute.String("client_id", clientID),
	)
}
