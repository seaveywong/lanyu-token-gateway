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
PostgreSQL: API Key、套餐、余额账本、用量日志、渠道配置
Redis: 限速、余额快照、渠道健康、缓存
```

## 表设计初稿

```text
ApiPlan
ApiKey
ApiChannel
ApiChannelCredential
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
