package config

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func SetupTracing(logger *log.Logger) (func(context.Context) error, error) {
	if !utils.IsTracingEnabled() {
		return nil, nil
	}

	serviceName := utils.OTelServiceName()

	endpoint := utils.GetEnvTrimmedOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	hostport, urlPath, insecure, err := parseOTLPEndpoint(endpoint)

	if err != nil {
		return nil, err
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(hostport),
		otlptracehttp.WithURLPath(urlPath),
	}

	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("setup tracing exporter: %w", err)
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("setup tracing resource: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	logger.Info("OpenTelemetry tracing enabled", "service", serviceName, "endpoint", endpoint)

	return tp.Shutdown, nil
}

func parseOTLPEndpoint(raw string) (hostport string, urlPath string, insecure bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false, fmt.Errorf("empty OTLP endpoint")
	}

	// Accept either:
	// - http(s)://host:port[/path]
	// - host:port
	if strings.Contains(raw, "://") {
		u, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", false, fmt.Errorf("invalid OTLP endpoint %q: %w", raw, parseErr)
		}
		if u.Host == "" {
			return "", "", false, fmt.Errorf("invalid OTLP endpoint %q: missing host", raw)
		}
		path := u.EscapedPath()
		if path == "" || path == "/" {
			path = "/v1/traces"
		}
		return u.Host, path, strings.EqualFold(u.Scheme, "http"), nil
	}

	// host:port
	return raw, "/v1/traces", true, nil
}
