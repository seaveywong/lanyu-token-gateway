// Package observability provides OpenTelemetry tracing, metrics, and
// structured logging initialization for Lanyu Token Gateway services.
//
// It sets up OTLP exporters for traces and metrics, configures a JSON-formatted
// slog logger, and returns a shutdown function for graceful teardown.
//
// Usage:
//
//	shutdown, err := observability.Init(ctx, observability.ObservabilityConfig{
//	    ServiceName:    "data-plane",
//	    ServiceVersion: "1.0.0",
//	    OTLPEndpoint:   "localhost:4317",
//	    LogLevel:       "info",
//	    LogFormat:      "json",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown(ctx)
//
//	logger := observability.Logger()
//	tracer := observability.Tracer("my-component")
//	meter  := observability.Meter("my-component")
package observability
