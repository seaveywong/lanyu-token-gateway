package observability

import (
	"context"
	"testing"
)

func TestInit_Success(t *testing.T) {
	cfg := ObservabilityConfig{
		ServiceName:    "test-service",
		ServiceVersion: "0.0.0",
		OTLPEndpoint:   "localhost:4317",
		LogLevel:       "debug",
		LogFormat:      "json",
	}

	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil after Init")
	}

	tracer := Tracer("test-component")
	if tracer == nil {
		t.Fatal("Tracer() returned nil after Init")
	}

	meter := Meter("test-component")
	if meter == nil {
		t.Fatal("Meter() returned nil after Init")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown() error = %v", err)
	}

	Reset()
}

func TestInit_TextFormat(t *testing.T) {
	cfg := ObservabilityConfig{
		ServiceName:    "test-text",
		ServiceVersion: "0.0.0",
		OTLPEndpoint:   "",
		LogLevel:       "warn",
		LogFormat:      "text",
	}

	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	_ = shutdown(context.Background())

	Reset()
}

func TestLogger_WithoutInit(t *testing.T) {
	Reset()
	logger := Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil without Init")
	}
}
