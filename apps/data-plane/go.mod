module github.com/seaveywong/lanyu-token-gateway/apps/data-plane

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/redis/go-redis/v9 v9.7.3
	gopkg.in/yaml.v3 v3.0.1
	github.com/seaveywong/lanyu-token-gateway/packages/apikey v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/config v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/contracts v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/observability v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk v0.0.0
)

replace (
	github.com/seaveywong/lanyu-token-gateway/packages/apikey => ../../packages/apikey
	github.com/seaveywong/lanyu-token-gateway/packages/config => ../../packages/config
	github.com/seaveywong/lanyu-token-gateway/packages/contracts => ../../packages/contracts
	github.com/seaveywong/lanyu-token-gateway/packages/observability => ../../packages/observability
	github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk => ../../packages/provider-sdk
)
