// Package telemetry configures OpenTelemetry export for Prompt Gate runtimes.
package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"promptgate/backend/internal/platform/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Provider owns an optional SDK tracer provider and its shutdown hook.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
}

// NewProvider builds and installs the global tracer provider when export is enabled.
func NewProvider(ctx context.Context, cfg config.OTelConfig, logger *slog.Logger) (*Provider, error) {
	provider := &Provider{}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.Enabled {
		return provider, nil
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
		otlptracehttp.WithTimeout(cfg.ExportTimeout),
		otlptracehttp.WithHeaders(otelHeaders(cfg)),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if cfg.CAFile != "" {
		tlsConfig, err := tlsConfigFromCAFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlptracehttp.WithTLSClientConfig(tlsConfig))
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("initialize OTLP HTTP exporter: %w", err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", serviceVersion()),
		attribute.String("deployment.environment.name", cfg.Environment),
		attribute.String("openinference.project.name", cfg.ProjectName),
	))
	if err != nil {
		return nil, fmt.Errorf("initialize OpenTelemetry resource: %w", err)
	}
	provider.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
			sdktrace.WithExportTimeout(cfg.ExportTimeout),
			sdktrace.WithMaxQueueSize(cfg.MaxQueueSize),
			sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatchSize),
		),
	)
	otel.SetTracerProvider(provider.tracerProvider)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error("OpenTelemetry export failed", "error", err)
	}))
	return provider, nil
}

func serviceVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || strings.TrimSpace(info.Main.Version) == "" {
		return "unknown"
	}
	return info.Main.Version
}

func otelHeaders(cfg config.OTelConfig) map[string]string {
	headers := map[string]string{"x-project-name": cfg.ProjectName}
	if cfg.APIKey != "" {
		headers["authorization"] = "Bearer " + cfg.APIKey
	}
	return headers
}

func tlsConfigFromCAFile(path string) (*tls.Config, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OpenTelemetry CA file: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("parse OpenTelemetry CA file %q", strings.TrimSpace(path))
	}
	return &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, nil
}

// Shutdown flushes queued spans. It is a no-op when telemetry is disabled.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.tracerProvider == nil {
		return nil
	}
	return p.tracerProvider.Shutdown(ctx)
}
