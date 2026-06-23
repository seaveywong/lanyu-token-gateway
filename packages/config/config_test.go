package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidFile(t *testing.T) {
	// Create a temporary YAML config file.
	content := []byte(`
server:
  port: "9090"
database:
  host: "db.example.com"
  port: "5432"
  user: "app"
  password: "secret"
  dbname: "lanyu"
redis:
  addr: "redis.example.com:6379"
observability:
  service_name: "test-service"
auth:
  pepper_path: "/etc/lanyu/pepper"
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9090" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "9090")
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.example.com")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for missing file, got nil")
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: "8080"},
		Database: DatabaseConfig{
			Host:   "localhost",
			Port:   "5432",
			User:   "app",
			DBName: "lanyu",
		},
		Redis: RedisConfig{Addr: "localhost:6379"},
		Observability: ObservabilityConfig{ServiceName: "test"},
		Auth: AuthConfig{PepperPath: "/etc/lanyu/pepper"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_MissingServerPort(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for missing server.port, got nil")
	}
}

func TestDefaults_Applied(t *testing.T) {
	data := []byte(`
server:
  port: "8080"
database:
  host: "localhost"
  port: "5432"
  user: "app"
  dbname: "test"
redis:
  addr: "localhost:6379"
observability:
  service_name: "test-svc"
auth:
  pepper_path: "/etc/pepper"
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != DefaultServerHost {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, DefaultServerHost)
	}
	if cfg.Database.MaxConns != DefaultDatabaseMaxConns {
		t.Errorf("Database.MaxConns = %d, want default %d", cfg.Database.MaxConns, DefaultDatabaseMaxConns)
	}
	if cfg.Redis.PoolSize != DefaultRedisPoolSize {
		t.Errorf("Redis.PoolSize = %d, want default %d", cfg.Redis.PoolSize, DefaultRedisPoolSize)
	}
	if cfg.Routing.DefaultRateLimit != DefaultRoutingRateLimit {
		t.Errorf("Routing.DefaultRateLimit = %d, want default %d", cfg.Routing.DefaultRateLimit, DefaultRoutingRateLimit)
	}
}
