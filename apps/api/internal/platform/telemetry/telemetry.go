// Package telemetry wires OpenTelemetry. With no endpoint it is a no-op, so
// local development and tests do not need a collector.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config controls the exporter.
type Config struct {
	Endpoint    string // OTLP/HTTP endpoint; empty disables tracing
	ServiceName string
	Insecure    bool
}

// Init installs the global tracer provider and returns a shutdown function.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: exporter: %w", err)
	}
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)))
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}
