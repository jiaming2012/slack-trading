package telemetry

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func Run(endpoint string, user string, apiToken string) {

	// Create the basic auth header
	authHeader := "Basic " + basicAuth(user, apiToken)

	fmt.Println("authHeader: ", authHeader)

	// Create a new HTTP exporter with basic authentication
	ctx := context.Background()
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": authHeader,
		}),
		otlptracehttp.WithURLPath("/traces"),
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	// Create a new tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("my-service"),
		)),
	)
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown TracerProvider: %v", err)
		}
	}()

	// Set the global tracer provider
	otel.SetTracerProvider(tp)

	// Create a new tracer
	tracer := otel.Tracer("example.com/trace")

	// Start a new span
	ctx, span := tracer.Start(ctx, "main")
	defer span.End()

	// Simulate some work
	time.Sleep(2 * time.Second)
}

// basicAuth returns the base64 encoded username:password for HTTP basic authentication
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
