module github.com/seaveywong/lanyu-token-gateway/apps/async-worker

go 1.24

require (
	github.com/seaveywong/lanyu-token-gateway/packages/config v0.0.0
	github.com/seaveywong/lanyu-token-gateway/packages/observability v0.0.0
)

replace (
	github.com/seaveywong/lanyu-token-gateway/packages/config => ../../packages/config
	github.com/seaveywong/lanyu-token-gateway/packages/observability => ../../packages/observability
)
