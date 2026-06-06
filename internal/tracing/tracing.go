package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds tracing configuration.
type Config struct {
	Enabled      bool
	ServiceName  string
	ExporterType string // otlp, jaeger, stdout
	Endpoint     string
	Insecure     bool
}

// Provider is the OpenTelemetry trace provider.
var Provider *sdktrace.TracerProvider

// Init initializes OpenTelemetry tracing.
func Init(ctx context.Context, cfg Config) error {
	if !cfg.Enabled {
		return nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch cfg.ExporterType {
	case "otlp", "otlphttp":
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(),
		)
	case "stdout", "":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		return fmt.Errorf("unknown exporter type: %s", cfg.ExporterType)
	}
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	Provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(Provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// Shutdown gracefully shuts down the trace provider.
func Shutdown(ctx context.Context) error {
	if Provider != nil {
		return Provider.Shutdown(ctx)
	}
	return nil
}

// Tracer returns the application tracer.
func Tracer() trace.Tracer {
	return otel.Tracer("datajobs")
}

// StartSpan starts a new span with the given name and attributes.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// AddAttributes adds attributes to the current span.
func AddAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(attrs...)
	}
}

// RecordError records an error on the current span.
func RecordError(ctx context.Context, err error) {
	if span := trace.SpanFromContext(ctx); span != nil {
		span.RecordError(err)
	}
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}