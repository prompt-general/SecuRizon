package telemetry

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
    "go.opentelemetry.io/otel/trace"
    "log"
    "os"
)

type TracingConfig struct {
    ServiceName    string
    ServiceVersion string
    Environment    string
    JaegerEndpoint string
    SampleRate     float64
}

func InitTracing(config TracingConfig) (func(context.Context) error, error) {
    // Create Jaeger exporter
    exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint(config.JaegerEndpoint),
    ))
    if err != nil {
        return nil, err
    }

    // Create resource with service attributes
    res := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(config.ServiceName),
        semconv.ServiceVersionKey.String(config.ServiceVersion),
        attribute.String("environment", config.Environment),
        attribute.String("host.name", getHostname()),
    )

    // Create tracer provider with batcher, resource, and sampler
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.ParentBased(
            sdktrace.TraceIDRatioBased(config.SampleRate),
        )),
    )

    otel.SetTracerProvider(tp)

    // Return shutdown function for graceful termination
    return tp.Shutdown, nil
}

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
    tracer := otel.Tracer("securazion")
    return tracer.Start(ctx, name, opts...)
}

func AddSpanAttributes(span trace.Span, attributes map[string]string) {
    attrs := make([]attribute.KeyValue, 0, len(attributes))
    for k, v := range attributes {
        attrs = append(attrs, attribute.String(k, v))
    }
    span.SetAttributes(attrs...)
}

func RecordSpanError(span trace.Span, err error, attributes map[string]string) {
    span.RecordError(err)
    if attributes != nil {
        AddSpanAttributes(span, attributes)
    }
    // Use otel codes for error status
    span.SetStatus(trace.StatusCodeError, err.Error())
}

func getHostname() string {
    h, err := os.Hostname()
    if err != nil {
        log.Printf("Failed to get hostname: %v", err)
        return "unknown"
    }
    return h
}
