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

// Tracer is the application tracer.
var tracer trace.Tracer

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

	tracer = Provider.Tracer(cfg.ServiceName)
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
	if tracer == nil {
		return otel.Tracer("datajobs")
	}
	return tracer
}

// StartSpan starts a new span with the given name.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// AddSpanAttributes adds attributes to the current span.
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span.
func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// WithJobID adds job ID to the span context.
func WithJobID(ctx context.Context, jobID string) context.Context {
	_, span := Tracer().Start(ctx, "job execution")
	span.SetAttributes(attribute.String("job.id", jobID))
	span.End()
	return ctx
}