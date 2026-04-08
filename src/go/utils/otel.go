package utils

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
)

// SetupOTelSDK bootstraps the OpenTelemetry pipeline with OTLP HTTP exporters
// for both traces and metrics. It configures the global TracerProvider,
// MeterProvider, and TextMapPropagator. The returned shutdown function must
// be called for proper cleanup (e.g., via defer).
//
// Configuration is controlled via OTEL_* environment variables (e.g.,
// OTEL_EXPORTER_OTLP_ENDPOINT). No endpoints are hardcoded.
func SetupOTelSDK(ctx context.Context, serviceName, serviceVersion string) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Set W3C TraceContext + Baggage propagation
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	// Create resource with service name, version, and instance ID (hostname)
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.ServiceInstanceID(hostname),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set up trace exporter (OTLP HTTP, endpoint from OTEL_* env vars)
	traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient())
	if err != nil {
		return nil, err
	}

	// Set up TracerProvider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up metric exporter (OTLP HTTP, endpoint from OTEL_* env vars)
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		handleErr(err)
		return
	}

	// Set up MeterProvider
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Set up log exporter (OTLP HTTP, endpoint from OTEL_* env vars)
	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		handleErr(err)
		return
	}

	// Set up LoggerProvider
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	otelglobal.SetLoggerProvider(loggerProvider)

	// Start Go runtime metrics collection
	err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		handleErr(err)
		return
	}

	return
}
