package observability

import (
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

// ---------------------------------------------------------------------------
// Global metric instruments — initialized by InitMetrics() and available
// for use throughout the application.
//
// Label policy:
//   - Metrics MUST NOT contain high-cardinality labels (API keys, user emails,
//     full prompt text, raw request bodies).
//   - Acceptable labels: "provider" (openai/anthropic/gemini), "model",
//     "status_code" (200/429/500), "service" (data-plane/control-plane),
//     "action" (route pattern), "channel" (channel id).
// ---------------------------------------------------------------------------

var (
	// RequestCounter counts total API requests by service and status_code.
	RequestCounter metric.Int64Counter

	// RequestDuration records request latency in milliseconds.
	RequestDuration metric.Float64Histogram

	// ActiveSSEConnections tracks the number of active server-sent-event (streaming)
	// connections. Goes up on connection open, down on close.
	ActiveSSEConnections metric.Int64UpDownCounter

	// TokenCounter counts tokens processed (input + output) by provider and model.
	TokenCounter metric.Int64Counter

	// RevenueCounter tracks revenue in micro_usd by organization.
	// Revenue is the amount charged to customers.
	RevenueCounter metric.Int64Counter

	// CostCounter tracks upstream provider costs in micro_usd by provider and model.
	CostCounter metric.Int64Counter

	// ChannelHealthGauge reports channel health per source.
	// 1 = healthy, 0 = unhealthy.
	ChannelHealthGauge metric.Int64Gauge

	// CircuitBreakerGauge reports circuit breaker state per source.
	// 0 = closed (normal), 1 = half-open (testing), 2 = open (blocked).
	CircuitBreakerGauge metric.Int64Gauge

	// LedgerBalanceGauge reports current ledger balance in micro_usd per wallet.
	LedgerBalanceGauge metric.Int64Gauge
)

// InitMetrics initializes all global metric instruments using the provided meter.
func InitMetrics(meter metric.Meter) error {
	var err error

	RequestCounter, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("init RequestCounter: %w", err)
	}

	RequestDuration, err = meter.Float64Histogram(
		"http_request_duration_ms",
		metric.WithDescription("HTTP request latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("init RequestDuration: %w", err)
	}

	ActiveSSEConnections, err = meter.Int64UpDownCounter(
		"sse_connections_active",
		metric.WithDescription("Number of active SSE streaming connections"),
	)
	if err != nil {
		return fmt.Errorf("init ActiveSSEConnections: %w", err)
	}

	TokenCounter, err = meter.Int64Counter(
		"tokens_processed_total",
		metric.WithDescription("Total number of tokens processed (input + output)"),
	)
	if err != nil {
		return fmt.Errorf("init TokenCounter: %w", err)
	}

	RevenueCounter, err = meter.Int64Counter(
		"revenue_micro_usd_total",
		metric.WithDescription("Total revenue in micro USD"),
		metric.WithUnit("micro_usd"),
	)
	if err != nil {
		return fmt.Errorf("init RevenueCounter: %w", err)
	}

	CostCounter, err = meter.Int64Counter(
		"upstream_cost_micro_usd_total",
		metric.WithDescription("Total upstream provider cost in micro USD"),
		metric.WithUnit("micro_usd"),
	)
	if err != nil {
		return fmt.Errorf("init CostCounter: %w", err)
	}

	ChannelHealthGauge, err = meter.Int64Gauge(
		"channel_health_gauge",
		metric.WithDescription("Channel health status: 1=healthy, 0=unhealthy"),
	)
	if err != nil {
		return fmt.Errorf("init ChannelHealthGauge: %w", err)
	}

	CircuitBreakerGauge, err = meter.Int64Gauge(
		"circuit_breaker_gauge",
		metric.WithDescription("Circuit breaker state: 0=closed, 1=half_open, 2=open"),
	)
	if err != nil {
		return fmt.Errorf("init CircuitBreakerGauge: %w", err)
	}

	LedgerBalanceGauge, err = meter.Int64Gauge(
		"ledger_balance_gauge",
		metric.WithDescription("Current ledger balance in micro USD"),
		metric.WithUnit("micro_usd"),
	)
	if err != nil {
		return fmt.Errorf("init LedgerBalanceGauge: %w", err)
	}

	return nil
}
