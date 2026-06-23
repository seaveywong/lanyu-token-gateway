package observability

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// ObservabilityConfig holds the configuration for initializing observability
// components (tracing, metrics, structured logging).
type ObservabilityConfig struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
	LogLevel       string
	LogFormat      string
}

// global references set during Init.
var (
	globalLogger *slog.Logger
	globalTP     trace.TracerProvider
	globalMP     metric.MeterProvider
)

// Init initializes the observability stack:
//
//   - Configures a structured JSON logger writing to stdout.
//   - Sets up OTLP trace and metric exporters (placeholder; real exporters
//     will be wired in when dependencies are added).
//   - Sets the global trace provider and meter provider.
//
// Returns a shutdown function that should be deferred by the caller.
func Init(ctx context.Context, cfg ObservabilityConfig) (shutdown func(context.Context) error, err error) {
	// --- Structured logger ---
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	globalLogger = slog.New(handler).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
	)
	slog.SetDefault(globalLogger)

	// --- Trace provider (placeholder) ---
	// In production this would create an OTLP gRPC/HTTP exporter.
	// For now we set a no-op provider; real wiring is added with dependencies.
	globalTP = tracenoop.NewTracerProvider()
	otel.SetTracerProvider(globalTP)

	// --- Meter provider (placeholder) ---
	globalMP = noop.NewMeterProvider()
	otel.SetMeterProvider(globalMP)

	globalLogger.InfoContext(ctx, "observability initialized",
		slog.String("otlp_endpoint", cfg.OTLPEndpoint),
	)

	// shutdown cleans up exporters.
	shutdown = func(ctx context.Context) error {
		globalLogger.InfoContext(ctx, "observability shutting down")
		return nil
	}

	return shutdown, nil
}

// Logger returns the global structured logger initialized by Init.
// If Init has not been called it falls back to the default slog logger.
func Logger() *slog.Logger {
	if globalLogger != nil {
		return globalLogger
	}
	return slog.Default()
}

// Tracer returns an OpenTelemetry tracer for the given instrumentation name.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// Meter returns an OpenTelemetry meter for the given instrumentation name.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}

// SetLogger allows tests or external code to inject a custom logger.
// This is primarily intended for testing.
func SetLogger(l *slog.Logger) {
	globalLogger = l
	slog.SetDefault(l)
}

// Reset restores all global state to defaults. Intended for tests.
func Reset() {
	globalLogger = nil
	globalTP = nil
	globalMP = nil
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	otel.SetTracerProvider(tracenoop.NewTracerProvider())
	otel.SetMeterProvider(noop.NewMeterProvider())
}

