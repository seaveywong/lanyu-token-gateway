# 架构草案

```text
Client
  -> Cloudflare WAF / Rate Limit
  -> api.lanyu.one
  -> Gateway Auth
  -> Balance / Rate Limit
  -> Model Mapping
  -> Cache Layer
  -> Channel Selector
  -> Provider Adapter
  -> Upstream Provider
```

## 数据存储规划

MVP 可以使用 SQLite，但必须抽象 Storage 层。

正式环境建议：

```text
PostgreSQL: API Key、套餐、余额账本、用量日志、渠道配置、
            订阅账号池凭证（subscription_accounts）
Redis: 限速、余额快照、渠道健康、订阅账号并发计数、缓存
```

## 账号池架构

账号来源分为四种类型，路由优先级如下：

1. `official_api_key` — 官方 API Key（最高优先）
2. `official_oauth` — 官方 OAuth 授权
3. `subscription_pool` — 订阅账号池（Plus/Pro/Team），池内多账号按"最少负载 + 加权轮询"选取
4. `upstream_api` — 合规上游 API（最后回退）

订阅池凭证通过后台 Cron 自动完成标准 OAuth Token 刷新，参考 New API / Sub2API
的实现模式。详见 `docs/ACCOUNT_SOURCE_ENTRY.md`。

## 表设计初稿

```text
ApiPlan
ApiKey
ApiChannel
ApiChannelCredential
ApiSubscriptionAccount
ApiModelMapping
ApiUsageLog
ApiBalanceLedger
ApiCacheEntry
ApiRateLimitRule
ApiAuditLog
```

## 与商城集成

商城支付成功后调用 Token Gateway 内部 API：

```text
POST /internal/api-keys
POST /internal/balance-ledgers
GET  /internal/usage-summary
```

商城不直接处理上游模型请求。
