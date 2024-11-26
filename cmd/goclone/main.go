package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"goclone/internal/api"
	"goclone/internal/config"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
    cfg config.Config
    tracer      trace.Tracer
)


func main() {
    cfg, err := config.LoadConfig(".")
    if err != nil {
        log.Fatal(errors.Wrap(err, "Failed to get config"))
    }

    if cfg.Core.OtlpEndpoint == "" {
        panic("OTLP endpoint is required")
    }

    ctx := context.Background()
    //exp, err := newConsoleExporter()
    exp, err := newOTLPExporter(ctx)
    if err != nil {
        log.Fatal(errors.Wrap(err, "Failed to create OTLP exporter"))
    }
    
    tp := newTraceProvider(exp)

    defer func() {_ = tp.Shutdown(ctx) }()
    
    otel.SetTracerProvider(tp)
    tracer = otel.Tracer("goclone")
    cfg.Core.Tracer = tracer

    fmt.Fprintln(os.Stdout, []any{"Starting Goclone"}...)
    api.StartAPI(cfg)
}

func newConsoleExporter() (oteltrace.SpanExporter, error) {
    return stdouttrace.New()
}

func newOTLPExporter(ctx context.Context) (oteltrace.SpanExporter, error) {
    insecureOpt := otlptracehttp.WithInsecure()
    endpointOpt := otlptracehttp.WithEndpoint(cfg.Core.OtlpEndpoint)
    return otlptracehttp.New(ctx, endpointOpt, insecureOpt)
}

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("goclone"),
		),
	)

	if err != nil {
		panic(err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}
