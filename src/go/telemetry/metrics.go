package telemetry

import (
	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
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
// MUST be called after utils.SetupOTelSDK().
func Init() error {
	return nil
}

// ShouldEmitOrderTelemetry returns true if order telemetry should be emitted
// for the given playground environment.
func ShouldEmitOrderTelemetry(env models.PlaygroundEnvironment) bool {
	return false
}

// PlaygroundAttrs returns OTel metric attributes for a playground.
func PlaygroundAttrs(env models.PlaygroundEnvironment, accountType models.LiveAccountType) metric.MeasurementOption {
	return nil
}
