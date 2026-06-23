package config

import "time"

// Server defaults.
const (
	DefaultServerHost           = "0.0.0.0"
	DefaultServerPort           = "8080"
	DefaultServerReadTimeout    = 30 * time.Second
	DefaultServerWriteTimeout   = 60 * time.Second
	DefaultServerShutdownTimout = 10 * time.Second
)

// Database defaults.
const (
	DefaultDatabaseHost       = "localhost"
	DefaultDatabasePort       = "5432"
	DefaultDatabaseSSLMode    = "disable"
	DefaultDatabaseMaxConns    = 50
	DefaultDatabaseMinConns    = 5
	DefaultDatabaseMaxLifetime = 30 * time.Minute
	DefaultDatabaseMaxIdleTime = 5 * time.Minute
)

// Redis defaults.
const (
	DefaultRedisAddr        = "localhost:6379"
	DefaultRedisDB          = 0
	DefaultRedisPoolSize    = 100
	DefaultRedisMinIdleConns = 10
	DefaultRedisDialTimeout  = 5 * time.Second
	DefaultRedisReadTimeout  = 3 * time.Second
	DefaultRedisWriteTimeout = 3 * time.Second
)

// Observability defaults.
const (
	DefaultLogLevel  = "info"
	DefaultLogFormat = "json"
)

// Auth defaults.
const (
	DefaultAuthKeyPrefix           = "ly_live_"
	DefaultAuthKeyByteLength       = 32
	DefaultAuthExpiryDuration      = 0 // 0 means no expiry by default
	DefaultAuthAccessTokenExpiry   = 15 * time.Minute
	DefaultAuthRefreshTokenExpiry  = 7 * 24 * time.Hour
)

// Routing defaults.
const (
	DefaultRoutingRateLimit     = 100
	DefaultRoutingConcurrency   = 10
	DefaultRoutingRetryBudget   = 3
)

// Billing defaults.
const (
	DefaultBillingPriceMicroUSD  int64  = 0
	DefaultBillingCreditPrecision       = 4
	DefaultBillingPlatformOrgID  string = "00000000-0000-0000-0000-000000000001"
)
