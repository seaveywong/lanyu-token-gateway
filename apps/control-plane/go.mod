module github.com/seaveywong/lanyu-token-gateway/apps/control-plane

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/jackc/pgx/v5 v5.7.2
	github.com/redis/go-redis/v9 v9.7.3
	github.com/seaveywong/lanyu-token-gateway/packages/auth v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/config v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/contracts v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/observability v0.0.0
	golang.org/x/crypto v0.37.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/seaveywong/lanyu-token-gateway/packages/apikey => ../../packages/apikey
	github.com/seaveywong/lanyu-token-gateway/packages/auth => ../../packages/auth
	github.com/seaveywong/lanyu-token-gateway/packages/config => ../../packages/config
	github.com/seaveywong/lanyu-token-gateway/packages/contracts => ../../packages/contracts
	github.com/seaveywong/lanyu-token-gateway/packages/observability => ../../packages/observability
)
