package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the Token Gateway services.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	Observability ObservabilityConfig `yaml:"observability"`
	Auth          AuthConfig          `yaml:"auth"`
	Routing       RoutingConfig       `yaml:"routing"`
	Billing       BillingConfig       `yaml:"billing"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           string        `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host        string        `yaml:"host"`
	Port        string        `yaml:"port"`
	User        string        `yaml:"user"`
	Password    string        `yaml:"password"`
	DBName      string        `yaml:"dbname"`
	SSLMode     string        `yaml:"sslmode"`
	MaxConns    int           `yaml:"max_conns"`
	MinConns    int           `yaml:"min_conns"`
	MaxLifetime time.Duration `yaml:"max_lifetime"`
	MaxIdleTime time.Duration `yaml:"max_idle_time"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr        string        `yaml:"addr"`
	Password    string        `yaml:"password"`
	DB          int           `yaml:"db"`
	PoolSize    int           `yaml:"pool_size"`
	MinIdleConns int          `yaml:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// ObservabilityConfig holds observability (tracing, metrics, logging) settings.
// This is the config-package version; it mirrors the observability package's
// ObservabilityConfig but lives here to keep the config YAML-taggable.
type ObservabilityConfig struct {
	ServiceName    string `yaml:"service_name"`
	ServiceVersion string `yaml:"service_version"`
	OTLPEndpoint   string `yaml:"otlp_endpoint"`
	LogLevel       string `yaml:"log_level"`
	LogFormat      string `yaml:"log_format"`
}

// AuthConfig holds authentication and API key settings.
type AuthConfig struct {
	PepperPath            string        `yaml:"pepper_path"`
	JWTSecret             string        `yaml:"jwt_secret"`
	AccessTokenExpiry     time.Duration `yaml:"access_token_expiry"`
	RefreshTokenExpiry    time.Duration `yaml:"refresh_token_expiry"`
	KeyPrefix             string        `yaml:"key_prefix"`
	KeyByteLength         int           `yaml:"key_byte_length"`
	DefaultExpiryDuration time.Duration `yaml:"default_expiry_duration"`
}

// RoutingConfig holds request routing defaults.
type RoutingConfig struct {
	DefaultRateLimit     int `yaml:"default_rate_limit"`
	DefaultConcurrency   int `yaml:"default_concurrency"`
	RetryBudget          int `yaml:"retry_budget"`
}

// BillingConfig holds billing and pricing defaults.
type BillingConfig struct {
	DefaultPriceMicroUSD int64 `yaml:"default_price_micro_usd"`
	CreditPrecision      int   `yaml:"credit_precision"`
}

// Load reads a YAML configuration file from the given path and returns a
// parsed Config. It applies defaults before returning.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	cfg.applyDefaults()
	return cfg, nil
}

// Validate checks that all required configuration fields are set and returns
// an error describing the first problem found.
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("config: server.port is required")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("config: database.host is required")
	}
	if c.Database.Port == "" {
		return fmt.Errorf("config: database.port is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("config: database.user is required")
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("config: database.dbname is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("config: redis.addr is required")
	}
	if c.Observability.ServiceName == "" {
		return fmt.Errorf("config: observability.service_name is required")
	}
	if c.Auth.PepperPath == "" {
		return fmt.Errorf("config: auth.pepper_path is required")
	}
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("config: auth.jwt_secret is required")
	}
	return nil
}

// applyDefaults fills in zero-value fields with sensible defaults.
func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = DefaultServerHost
	}
	if c.Server.Port == "" {
		c.Server.Port = DefaultServerPort
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = DefaultServerReadTimeout
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = DefaultServerWriteTimeout
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = DefaultServerShutdownTimout
	}

	if c.Database.Host == "" {
		c.Database.Host = DefaultDatabaseHost
	}
	if c.Database.Port == "" {
		c.Database.Port = DefaultDatabasePort
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = DefaultDatabaseSSLMode
	}
	if c.Database.MaxConns == 0 {
		c.Database.MaxConns = DefaultDatabaseMaxConns
	}
	if c.Database.MinConns == 0 {
		c.Database.MinConns = DefaultDatabaseMinConns
	}
	if c.Database.MaxLifetime == 0 {
		c.Database.MaxLifetime = DefaultDatabaseMaxLifetime
	}
	if c.Database.MaxIdleTime == 0 {
		c.Database.MaxIdleTime = DefaultDatabaseMaxIdleTime
	}

	if c.Redis.Addr == "" {
		c.Redis.Addr = DefaultRedisAddr
	}
	if c.Redis.PoolSize == 0 {
		c.Redis.PoolSize = DefaultRedisPoolSize
	}
	if c.Redis.MinIdleConns == 0 {
		c.Redis.MinIdleConns = DefaultRedisMinIdleConns
	}
	if c.Redis.DialTimeout == 0 {
		c.Redis.DialTimeout = DefaultRedisDialTimeout
	}
	if c.Redis.ReadTimeout == 0 {
		c.Redis.ReadTimeout = DefaultRedisReadTimeout
	}
	if c.Redis.WriteTimeout == 0 {
		c.Redis.WriteTimeout = DefaultRedisWriteTimeout
	}

	if c.Observability.LogLevel == "" {
		c.Observability.LogLevel = DefaultLogLevel
	}
	if c.Observability.LogFormat == "" {
		c.Observability.LogFormat = DefaultLogFormat
	}

	if c.Auth.AccessTokenExpiry == 0 {
		c.Auth.AccessTokenExpiry = DefaultAuthAccessTokenExpiry
	}
	if c.Auth.RefreshTokenExpiry == 0 {
		c.Auth.RefreshTokenExpiry = DefaultAuthRefreshTokenExpiry
	}
	if c.Auth.KeyPrefix == "" {
		c.Auth.KeyPrefix = DefaultAuthKeyPrefix
	}
	if c.Auth.KeyByteLength == 0 {
		c.Auth.KeyByteLength = DefaultAuthKeyByteLength
	}
	if c.Auth.DefaultExpiryDuration == 0 {
		c.Auth.DefaultExpiryDuration = DefaultAuthExpiryDuration
	}

	if c.Routing.DefaultRateLimit == 0 {
		c.Routing.DefaultRateLimit = DefaultRoutingRateLimit
	}
	if c.Routing.DefaultConcurrency == 0 {
		c.Routing.DefaultConcurrency = DefaultRoutingConcurrency
	}
	if c.Routing.RetryBudget == 0 {
		c.Routing.RetryBudget = DefaultRoutingRetryBudget
	}

	if c.Billing.CreditPrecision == 0 {
		c.Billing.CreditPrecision = DefaultBillingCreditPrecision
	}
}
