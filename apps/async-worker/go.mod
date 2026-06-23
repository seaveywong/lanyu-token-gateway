module github.com/seaveywong/lanyu-token-gateway/apps/async-worker

go 1.24

require (
	github.com/jackc/pgx/v5 v5.7.2
	github.com/redis/go-redis/v9 v9.7.3
	github.com/seaveywong/lanyu-token-gateway/packages/config v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/observability v0.0.0
)

replace (
	github.com/seaveywong/lanyu-token-gateway/packages/config => ../../packages/config
	github.com/seaveywong/lanyu-token-gateway/packages/observability => ../../packages/observability
)
