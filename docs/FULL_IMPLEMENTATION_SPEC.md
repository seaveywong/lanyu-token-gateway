# 蓝域 Token Gateway 完整实现规格

> 文档状态：开发主规格
>
> 适用仓库：`seaveywong/lanyu-token-gateway`
>
> 目标读者：产品负责人、架构师、后端、前端、运维、安全、测试

本文件定义可上线运营的完整形态。开发人员应将本文的 `MUST` 作为强制验收项，
将 `SHOULD` 作为上线前应完成项。未在本文明确的接口、数据字段和第三方能力，
必须先通过 ADR（Architecture Decision Record）留痕后再实现。

相关文档：

- `docs/ARCHITECTURE.md`：架构摘要。
- `docs/AI_INTEGRATION_PLAN.md`：AI 对接流程。
- `docs/ACCOUNT_SOURCE_ENTRY.md`：账号来源管理边界。
- `docs/RISK_BOUNDARIES.md`：合规与安全边界。

---

## 1. 产品定义与边界

### 1.1 产品定位

蓝域 Token Gateway 是独立于 `D:\dev\FB` 资产商城的多厂商 AI API 网关和运营平台。
它为客户提供统一、可审计、可计费的 API 访问层，并为运营人员提供渠道、模型、
密钥、用量、账本和风控管理能力。

产品的核心价值：

- 对客户：一个 API Key、统一文档、稳定的模型调用、明确余额和用量。
- 对运营：自有官方 API 资源优先、合规上游自动回退、可控成本与完整审计。
- 对工程：通过适配器支持 OpenAI、Anthropic、Google Gemini 等热门厂商，避免业务层
  被任何单一厂商协议锁死。

### 1.2 能力边界与风险分级

平台能力按风险等级分为三级管理：

#### P0 严格禁止

以下能力不得开发、不得以隐藏配置实现、不得写入运维脚本：

- 不得伪造厂商 API、绕过厂商访问控制、隐藏用户真实用量或规避服务条款。
- 不得自动化注册账号、自动化订阅购买、信用卡/支付欺诈。
- 不得将客户提示词、授权头、原始 API Key、支付回调签名写入日志、埋点或告警。
- 平台不移入浏览器引擎（Selenium / Puppeteer / Playwright）；Token 提取由
  运营人员在本地浏览器完成（DevTools / HAR 导出），平台只接收已提取的 Token。

#### P1 受控开放

以下能力对标开源社区成熟方案（New API 25k+ / Sub2API 21k+ / chat2api 18k+
Stars），在平台中完整实现：

- **订阅账号池 (`subscription_pool`)**：企业持有的 Plus/Pro/Team 订阅账号集合，
  支持以下全自动化能力：
  - **凭证导入**：支持手动粘贴 Session Token / Refresh Token / Access Token；
    支持上传 HAR 归档文件（平台自动解析提取 Token）；支持导入 Cookie JSON。
    原始 HAR 内存解析后即时加密，不落盘。
  - **OAuth 自动刷新**：后台 Cron 定时扫描，Token 到期前通过 `/oauth/token`
    端点自动刷新。支持 `session_token` → OAuth 交换 → `access_token` +
    `refresh_token` 的完整流程。
  - **Cookie 保活**：对无标准 OAuth 端点的 Provider，定期 HTTP 请求维持
    Cookie 会话有效性。
  - **Arkose / CAPTCHA 自动打码**：集成 YesCaptcha / CapSolver / 2Captcha，
    OAuth 端点触发验证时自动完成。Solver 支持多供应商冗余和自动切换。
    打码失败时账号进入 `manual_intervention`，并通知运营人员。
  - **代理绑定**：每账号可绑定 HTTP / SOCKS5 / 住宅代理；代理 IP 支持
    static / per_request / per_session 三种轮换策略。
  - **池内调度**：最少负载优先 + 加权轮询。每账号独立配额窗口、冷却时间、
    故障计数和状态机。
  - **客户透明**：路由模拟器和审计日志中标注 `source_type=subscription_pool`。
  - 若供应商服务条款明确禁止 Token 共享或非官方 API 访问，该供应商标注为
    "评估中"，由运营方自行决定是否启用。

#### P2 评估中

以下能力仅做合规评估文档，不进入当前开发规划：

- 用户自有 Key 托管（BYOK）：客户将个人 API Key 托管到平台。
- OAuth 设备码流：客户通过设备码授权平台访问其订阅资源。

---

“自有号池”在本项目中表示：

- 企业合法持有的官方 API 项目密钥、服务账号或官方 OAuth 授权；
- 企业合法持有的 Plus/Pro/Team 订阅账号的 Session/Refresh Token
  （作为 `subscription_pool` 类型管理，通过标准 OAuth 自动刷新）；
- 通过正式合同获取的合规上游 API 接口。

若某供应商没有正式 API 授权渠道且其服务条款明确禁止 Token 共享或非官方
API 访问，则该供应商不能纳入池。

### 1.3 域名与前后端分离

建议域名如下，所有域名接入 Cloudflare：

| 域名 | 用途 | 是否公开 API |
| --- | --- | --- |
| `api.lanyu.one` | 客户调用入口 | 是 |
| `console.lanyu.one` | 运营后台与客户门户 | 否，仅网页 |
| `docs.api.lanyu.one` | 开发者文档与状态页入口 | 否，仅网页 |
| `origin-token.internal` | 控制面和数据面真实源站标识 | 否，不配置公共 DNS |

浏览器只访问 `console.lanyu.one` 和其同源 `/admin-api/*`、`/portal-api/*` 代理路径。
真实控制面源站只接受 Cloudflare Tunnel 或私网流量。客户 SDK 只看到 `api.lanyu.one`，
任何 API 响应、错误、CORS、重定向、静态资源和日志中都不得泄漏源站地址、渠道名、
上游域名或渠道密钥指纹。

---

## 2. 角色、租户与业务流程

### 2.1 多租户模型

平台采用“组织（Organization）- 项目（Project）- 成员（Member）- API Key”模型：

- 一个用户可属于多个组织。
- 一个组织可有多个项目，项目是密钥、预算、模型权限和用量统计的最小隔离单元。
- 一个 API Key 必须归属于唯一项目。
- 所有业务表必须带 `organization_id`，需要项目隔离的表再带 `project_id`。
- 后端必须在服务层强制租户范围，PostgreSQL 生产环境必须增加 RLS 作为第二道防线。

### 2.2 角色定义

| 角色 | 主要权限 |
| --- | --- |
| `platform_owner` | 平台级不可转移权限、密钥策略、灾难恢复审批 |
| `platform_admin` | 用户、渠道、模型、价格、风控、全局配置 |
| `operator` | 渠道健康、路由规则、模型映射、故障处理 |
| `finance` | 充值、退款、账本、支付渠道、对账、发票状态 |
| `support` | 工单、用户诊断、有限只读用量，不能查看密钥或渠道秘密 |
| `org_owner` | 组织成员、项目、预算、组织账单 |
| `org_admin` | 组织项目、成员和 API Key 管理 |
| `developer` | 创建/轮换本项目 API Key、查看本项目日志 |
| `viewer` | 只读项目数据 |

高风险权限必须拆分，`finance` 不能管理上游密钥，`operator` 不能提现或修改支付收款，
`support` 不能导出提示词、完整请求或任意密钥。

### 2.3 客户主流程

1. 用户注册、验证邮箱、启用 MFA，并创建或加入组织。
2. 组织创建项目，选择可用模型和预算上限。
3. 用户创建 API Key，系统仅在创建时展示一次明文。
4. 客户调用 `api.lanyu.one` 的兼容接口。
5. 网关完成鉴权、限流、额度预占、路由、流式转发和结算。
6. 客户在门户查看用量、账单、错误诊断、密钥状态和充值记录。
7. 发生异常时，客户创建工单；支持人员只能看到已脱敏的关联请求信息。

### 2.4 运营主流程

1. 管理员创建”账号来源”（官方 API Key、官方 OAuth、订阅账号池、合规上游 API）。
   订阅池需导入 Session/Refresh Token，平台自动完成 OAuth 交换和持续刷新。
2. 运营人员验证来源，绑定可用模型、区域、成本和预算。
3. 运营人员将来源加入渠道，设定优先级、权重、并发、熔断与回退策略。
4. 管理员维护平台模型目录和对外模型别名。
5. 管理员为组织、项目或 API Key 配置模型权限、倍率、预算和限流计划。
6. 网关按“自有官方来源优先，合规上游来源回退”的规则执行请求。
7. 财务根据不可变账本、支付回调和上游成本进行日对账。

---

## 3. 总体架构

### 3.1 逻辑分层

```text
Client SDK / Customer App
          |
Cloudflare DNS + WAF + DDoS + Rate Controls
          |
api.lanyu.one Edge Worker
          |
Cloudflare Tunnel (outbound-only)
          |
Data Plane API (Go, stateless, SSE capable)
          |
PostgreSQL / Redis / Provider Adapters / Official Provider APIs

console.lanyu.one -> Cloudflare Pages -> same-origin Worker proxy
          |
Cloudflare Tunnel
          |
Control Plane API (Go) -> PostgreSQL / Redis / Object Storage
          |
Async Worker -> Outbox / NATS JetStream / notifications / reconciliation
```

### 3.2 服务划分

| 服务 | 责任 | 是否面向公网 |
| --- | --- | --- |
| `edge-gateway` | 请求 ID、边缘防护、基础限额、隐藏源站、转发 | 仅 Cloudflare Worker |
| `data-plane` | API 鉴权、路由、调用、流式转发、用量结算 | 仅 Tunnel 后源站 |
| `control-plane` | 后台、门户、RBAC、密钥、渠道、价格、账本 | 仅 Tunnel 后源站 |
| `async-worker` | 对账、邮件、Webhook、缓存失效、报表、告警 | 不公开 |
| `scheduler` | 定时健康检查、预算检查、渠道校验、归档 | 不公开，可与 worker 合并 |
| `admin-web` | React/TypeScript 管理后台 | Cloudflare Pages |
| `portal-web` | React/TypeScript 客户门户 | Cloudflare Pages |

`data-plane` 与 `control-plane` 必须独立部署、独立限额、独立服务账号。后台高负载或
慢查询不得影响 API 转发。早期可在同一台服务器运行容器，但必须保留独立镜像、端口、
配置和扩容单元。

### 3.3 推荐技术栈

| 层 | 推荐方案 | 原因 |
| --- | --- | --- |
| 数据面与控制面 | Go，固定最新稳定大版本 | 高并发流式连接、内存稳定、单二进制部署 |
| HTTP 路由 | `chi` 或同等级轻量路由 | 标准库兼容、易测、无重框架绑定 |
| PostgreSQL 访问 | `pgx` + `sqlc` | 强类型 SQL、可审计查询、性能可控 |
| 数据迁移 | `goose` 或 Atlas | 版本化、可审计、支持非破坏性迁移 |
| Redis | Redis 7+ | 限流、并发计数、短期缓存、熔断状态 |
| 异步事件 | PostgreSQL Outbox + NATS JetStream | 账务可追溯，事件可重放 |
| 管理端/门户 | React + TypeScript + Vite + TanStack Query/Router | 类型共享、构建快、适合独立 SPA |
| API 契约 | OpenAPI 3.1 + JSON Schema | 生成客户端、接口可测试 |
| 可观测性 | OpenTelemetry + Prometheus + Grafana + Loki + Tempo/Sentry | 指标、日志、追踪和错误关联 |
| 部署 | Docker Compose 起步，Kubernetes 仅在需要多节点时引入 | 避免过早复杂化 |

不得把支付、渠道凭证、路由策略和账本逻辑写进 Cloudflare Worker。Worker 仅做边缘
防护与无状态转发；业务真相在控制面、数据面、PostgreSQL 中。

### 3.4 仓库结构

```text
Token/
  apps/
    data-plane/              # 客户 API 网关
    control-plane/           # 管理/门户 API
    async-worker/            # 异步任务与定时任务
    admin-web/               # 平台后台
    portal-web/              # 客户门户
    edge-gateway/            # Cloudflare Worker
  packages/
    contracts/               # OpenAPI、JSON Schema、共享错误码
    provider-sdk/            # 渠道适配器接口与测试夹具
    config/                  # 配置结构与校验
    observability/           # trace/log/metric 共用封装
  db/
    migrations/
    queries/
    seed/
  deploy/
    compose/
    cloudflare/
    systemd/
    runbooks/
  docs/
  .github/workflows/
  Makefile
  AGENTS.md
```

---

## 4. 对外 API 与兼容策略

### 4.1 原则

- 对外主协议为 OpenAI 风格 API，统一前缀为 `https://api.lanyu.one/v1`。
- 兼容必须以已声明的能力矩阵为准，不得假装某厂商功能已完全支持。
- 所有请求必须支持 `Authorization: Bearer <API_KEY>`；不接受 Query String Key。
- 除公开模型列表外，所有数据接口必须认证。
- 所有错误统一使用 JSON 错误信封，流式连接使用 SSE 错误事件。
- 客户可选原生厂商兼容入口，但内部必须转换为统一请求中间表示（Canonical IR）。

### 4.2 第一批公开接口

| 入口 | 优先级 | 说明 |
| --- | --- | --- |
| `GET /v1/models` | P0 | 返回当前 Key 有权调用的模型与能力 |
| `POST /v1/chat/completions` | P0 | 文本、多轮、工具调用、SSE |
| `POST /v1/responses` | P0 | 统一响应式 API、流式、工具调用 |
| `POST /v1/embeddings` | P0 | 向量接口 |
| `POST /v1/moderations` | P1 | 内容审核能力，按上游支持情况启用 |
| `POST /v1/images/generations` | P1 | 图像生成，异步任务或同步返回由能力定义 |
| `POST /v1/audio/speech` | P1 | 文本转语音 |
| `POST /v1/audio/transcriptions` | P1 | 语音转文本，必须限制文件大小与类型 |
| `POST /v1/batches` | P2 | 批任务，使用对象存储与异步状态机 |
| `GET /v1/usage` | P1 | 当前 Key/项目的可见用量摘要 |

### 4.3 原生兼容入口

为降低现有客户迁移成本，第二阶段实现以下入口。它们是兼容层，不是对外主文档：

| 前缀 | 目标协议 | 支持范围 |
| --- | --- | --- |
| `/anthropic/v1/messages` | Anthropic Messages | 文本、图像、工具、SSE；按能力表裁剪 |
| `/google/v1beta/models/{model}:generateContent` | Gemini GenerateContent | 文本、多模态、函数调用 |
| `/google/v1beta/models/{model}:streamGenerateContent` | Gemini 流式 | 流式事件映射 |

Azure、Cohere、Mistral、AWS Bedrock、Vertex AI 等厂商先作为内部 Provider Adapter 支持，
是否暴露原生 URL 取决于真实客户需求。不要为“看起来全面”而维护无用户使用的协议面。

### 4.4 Canonical IR

所有入站协议转换为以下抽象，Provider Adapter 不直接接收原始 HTTP Body：

```text
GenerationRequest
  request_id
  tenant/project/key context
  requested_model
  modality: text | image | audio | embedding | moderation
  messages / input
  tools and tool_choice
  response_format
  generation parameters
  stream flag
  idempotency key
  privacy/cache policy
```

输出转换为 `GenerationResponse` 或 `GenerationEvent`。每个字段必须标注：

- `native`：该上游原样支持。
- `translated`：网关可无损转换。
- `best_effort`：仅在客户显式允许时转换。
- `unsupported`：明确返回 `feature_not_supported`，不得静默忽略。

### 4.5 统一错误格式

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Request rate limit exceeded.",
    "type": "gateway_error",
    "request_id": "req_...",
    "retry_after_seconds": 30
  }
}
```

错误码至少包括：`invalid_api_key`、`key_disabled`、`insufficient_balance`、
`project_budget_exceeded`、`model_not_allowed`、`invalid_request`、
`request_too_large`、`rate_limit_exceeded`、`concurrency_limit_exceeded`、
`upstream_timeout`、`provider_unavailable`、`feature_not_supported`、
`idempotency_conflict`、`internal_error`。禁止回传内部堆栈、SQL、上游 URL、渠道 ID。

### 4.6 流式处理要求

- 使用 SSE，保持 `Content-Type: text/event-stream`。
- 首字节发出后不得切换到另一渠道重试，避免客户重复收到内容或被重复计费。
- 首字节前仅允许对可重试网络错误、429、明确的 5xx 执行最多一次安全回退。
- 客户主动断开后，应取消上游上下文；若上游不可取消，必须继续准确结算。
- 每 15 秒发送心跳，连接空闲、总时长、响应字节数必须可配置上限。

---

## 5. 渠道、模型与路由

### 5.1 实体定义

- `provider`：厂商类型，例如 OpenAI、Anthropic、Gemini、Bedrock。
- `account_source`：一组合法凭证或授权，类型包括：
  - `official_api_key`：官方 API 项目密钥。
  - `official_oauth`：官方 OAuth 授权。
  - `upstream_api`：合规上游 API。
  - `subscription_pool`：订阅账号池（Plus/Pro/Team），多个订阅账号的
    Session/Refresh Token 集合，通过标准 OAuth 自动刷新维持。
  详情见 `ACCOUNT_SOURCE_ENTRY.md`。
- `channel`：一个可被路由的逻辑渠道，可包含一个或多个账号来源。
- `model_catalog`：平台规范模型目录，含能力、输入/输出限制、模态与版本。
- `model_mapping`：外部模型别名到渠道原生模型名的映射。
- `route_rule`：租户、项目、模型、区域或标签条件下的路由规则。

### 5.2 Provider Adapter 契约

每个 Provider Adapter 必须实现以下接口，并为每个接口提供单元测试夹具：

```go
type ProviderAdapter interface {
    Provider() ProviderID
    Validate(ctx context.Context, source SourceCredential) ValidationResult
    DiscoverModels(ctx context.Context, source SourceCredential) []ProviderModel
    Capabilities(model string) ModelCapabilities
    Estimate(ctx context.Context, req CanonicalRequest) CostEstimate
    Invoke(ctx context.Context, req CanonicalRequest, source ResolvedSource) (CanonicalResponse, Usage, error)
    Stream(ctx context.Context, req CanonicalRequest, source ResolvedSource, emit EventSink) (Usage, error)
    NormalizeError(err error) ProviderError
    Health(ctx context.Context, source ResolvedSource) HealthResult
}
```

适配器必须只连接厂商公开、被授权的正式 API。渠道配置中的 Base URL 必须命中平台
维护的域名允许列表，不能让运营人员输入任意 URL 以防 SSRF。

### 5.3 路由算法

默认路由顺序：

1. 验证 API Key、项目状态、模型权限、速率、并发、余额和预算。
2. 解析外部模型别名，得到可用渠道候选集。
3. 过滤禁用、过期、超预算、健康异常、区域不匹配、并发耗尽的来源。
4. 优先选择自有官方来源（`official_api_key`、`official_oauth`）。
5. 自有订阅池（`subscription_pool`）在同优先级中排在官方 API/OAuth 之后，
   但在上游代理之前。选择池内当前配额可用、未冷却、状态为 active 的账号；
   池内账号按”最少负载优先 + 加权轮询”选取。若池内无可用账号，该池视为
   不可用，进入下一候选。
6. 在同优先级来源中按”优先级、剩余预算、健康分、并发余量、成本、权重”排序。
7. 自有来源无可用候选时，才选择已批准的合规上游来源（`upstream_api`）。
8. 在安全条件满足时执行一次回退；记录完整内部路由决策。
9. 根据实际输入/输出 Token 或上游账单数据完成结算。

路由不得仅使用简单轮询。每个候选来源必须有：

- `priority`：整数，数值越小优先级越高。
- `weight`：同优先级加权轮询权重。
- `max_concurrency`：当前来源最大活跃请求数。
- `daily_budget_micro_usd`：来源日成本上限。
- `failure_threshold`、`cooldown_seconds`：熔断策略。
- `allowed_models`、`allowed_regions`、`tenant_allowlist`：权限边界。

`subscription_pool` 来源额外拥有账号级路由属性（每行 `subscription_accounts`）：

- `quota_limit_per_window`、`quota_remaining`、`quota_window_seconds`：单账号配额窗口。
- `cooldown_until`：单账号冷却截止时间。
- `consecutive_failures`：连续失败次数，用于故障升级。
- `token_expires_at`：Token 过期时间，触发自动刷新判断。
- `status`：账号状态（active / cooldown / exhausted / dead / manual_disabled / manual_intervention）。

### 5.4 健康检查与熔断

- 主动健康检查每 60 秒执行一次，间隔可配置；禁止高频探测消耗昂贵模型。
- 被动健康检查从真实请求结果计算，区分认证失败、限流、余额不足、上游故障和格式错误。
- 连续可重试失败达到阈值后，渠道进入 `open` 熔断状态。
- 冷却期结束后只允许少量 `half_open` 探测请求。
- 凭证失效、余额不足、条款异常必须立即禁用，不等待普通熔断恢复。
- 管理后台必须能看到状态、最近错误分类、最后成功时间和人工启停记录。

### 5.5 缓存策略

第一阶段仅实施精确缓存（Exact Cache），不实施语义缓存：

- 缓存键包含租户、模型、模型版本、规范化输入、系统提示词、工具定义、温度、
  响应格式和网关协议版本的 HMAC 摘要。
- 默认不缓存；组织、项目或请求可显式启用。
- 含个人数据、密钥、文件、音频、图像、工具调用、流式响应默认不缓存。
- 缓存体使用对象存储或 Redis，必须加密并设置 TTL；不允许跨租户命中。
- 每次缓存命中仍记录用量事件，但按零或自定义缓存价格结算。
- 管理员可按组织、项目、模型或版本失效缓存，操作必须审计。

语义缓存仅在后续独立 ADR、客户授权、数据隔离评审、召回准确率评测通过后实施。

---

## 6. 身份、密钥与访问控制

### 6.1 用户登录

- 客户门户和后台使用 OIDC/OAuth 2.1 认证；优先支持企业 SSO（SAML/OIDC）与邮箱登录。
- 管理员、财务和平台 Owner 必须启用 TOTP 或 WebAuthn MFA。
- 会话 Cookie 必须为 `HttpOnly`、`Secure`、`SameSite=Lax/Strict`，后台操作使用 CSRF 防护。
- 登录、改密、MFA 重置、支付配置、渠道密钥轮换都必须触发风险审计和通知。
- 支持账户锁定、设备会话列表、全端退出、IP/地区异常告警。

### 6.2 API Key 规则

- Key 格式建议：`ly_live_<随机主体>`、`ly_test_<随机主体>`。
- 生成时使用至少 256 bit CSPRNG 随机性。
- 数据库只保存 `key_prefix` 与 `HMAC-SHA-256(pepper, raw_key)`；`pepper` 存放在 KMS 或
  运行环境密钥中，绝不进数据库。
- Key 明文只在创建时显示一次；后续仅显示前后缀、创建时间、最后使用时间和状态。
- Key 支持作用域、IP CIDR 白名单、过期时间、模型白名单、项目预算和独立速率限制。
- Key 支持轮换：旧 Key 可有最多 24 小时宽限期，且可提前立即失效。
- 禁止在 URL、浏览器 LocalStorage、前端构建变量、错误页和日志中出现 Key。

### 6.3 权限模型

- 控制面使用 RBAC + 资源范围（平台/组织/项目）校验。
- 高危操作采用双重确认：输入操作对象名称、重新验证 MFA、记录原因。
- 支付退款、渠道删除、密钥导出、组织所有权转移应支持四眼审批，可配置为强制。
- 每个 API Handler 必须声明权限常量；禁止通过前端隐藏按钮代替后端鉴权。

---

## 7. 计量、计费、支付与账本

### 7.1 金额模型

- 内部金额统一用整数最小单位 `micro_usd` 或内部 `credit`，绝不使用浮点数。
- 用户余额、冻结额度、收入、成本、优惠、退款均通过不可变账本表达。
- 账本采用双分录：每笔业务至少产生借方与贷方，二者合计必须为零。
- 价格表必须版本化，历史请求永远使用当时有效的价格版本，不得回写历史。

### 7.2 请求结算流程

1. 在调用前按模型、最大输出、图片/音频估算值预占余额。
2. 若余额或项目预算不足，直接拒绝，避免产生无资金保障的上游费用。
3. 调用结束后根据上游用量、网关 Token 计数或异步账单数据计算实际成本和售价。
4. 将预占转为最终扣费，多余预占立即释放。
5. 失败且上游未受理时全额释放；上游可能已受理时按可核验用量结算并标记待对账。
6. 每个结算事件必须以 `request_id` 和 `idempotency_key` 去重。

### 7.3 倍率与价格规则

价格规则按以下优先级解析：

1. 特定 API Key 覆盖。
2. 项目规则。
3. 组织套餐规则。
4. 平台模型默认规则。

规则应支持输入、输出、缓存命中、图像、音频、工具调用、批处理和渠道附加成本。
倍率不能直接覆盖历史账单；修改后生成新 `pricing_version` 并指定生效时间。

### 7.4 充值与支付

- 支付服务独立为 `billing` 模块，禁止支付回调直接改余额。
- 支持支付宝、微信支付应通过持牌/合规聚合支付接口或企业直连，不在浏览器暴露商户密钥。
- 回调必须校验签名、时间戳、金额、商户订单号和幂等状态。
- 支付订单状态机：`created -> pending -> paid -> credited`，另有 `expired`、`failed`、
  `refunded`、`manual_review`。
- 每笔成功支付由异步 Worker 创建账本分录；重复回调不得重复入账。
- 退款必须走审批、原路退款、账本冲正和审计；禁止直接修改余额字段。

### 7.5 对账

- 每日对上游厂商账单、渠道侧余额、平台请求记录、用户账本和支付订单进行核对。
- 差异分为延迟账单、计量差异、重复请求、未知扣费、支付差异。
- 差异进入工作队列，禁止脚本直接修正；修正必须以补充账本分录完成。
- 财务后台应导出按组织、项目、模型、渠道、日期的收入、成本、毛利和异常列表。

---

## 8. 数据库设计

### 8.1 强制表清单

| 域 | 表 |
| --- | --- |
| 身份与租户 | `users`, `organizations`, `organization_members`, `projects`, `roles`, `sessions`, `mfa_factors` |
| API 访问 | `api_keys`, `api_key_scopes`, `api_key_ip_rules`, `request_logs`, `usage_events`, `idempotency_records` |
| 渠道 | `providers`, `account_sources`, `subscription_accounts`, `channels`, `channel_sources`, `model_catalog`, `model_mappings`, `route_rules`, `channel_health_events` |
| 计费 | `wallets`, `ledger_accounts`, `ledger_entries`, `ledger_postings`, `pricing_versions`, `pricing_rules`, `usage_reservations` |
| 支付 | `payment_orders`, `payment_webhooks`, `refund_requests`, `reconciliation_runs`, `reconciliation_items` |
| 运维与安全 | `audit_logs`, `security_events`, `feature_flags`, `system_settings`, `outbox_events`, `webhook_endpoints`, `webhook_deliveries` |
| 客服 | `support_tickets`, `ticket_messages`, `ticket_attachments` |

### 8.2 关键字段要求

`account_sources`：

```text
id, name, source_type, provider_id, endpoint_id,
credential_ciphertext, credential_key_version, credential_fingerprint,
model_policy_json, priority, weight, max_concurrency,
daily_budget_micro_usd, subscription_accounts_count,
status, health_state,
last_validated_at, last_used_at, created_by, created_at, updated_at
```

`subscription_accounts`（仅 `subscription_pool` 类型使用）：

```text
id, source_id (FK -> account_sources), account_label,
credential_type,  -- session_token | refresh_token | access_token
credential_ciphertext, credential_key_version, credential_fingerprint,
refresh_ciphertext, refresh_key_version,
token_expires_at, quota_limit_per_window, quota_remaining,
quota_window_seconds, cooldown_until, consecutive_failures,
status,  -- active | cooldown | exhausted | dead | manual_disabled | manual_intervention
last_used_at, last_refreshed_at, last_error_code, last_error_message,
created_at, updated_at
```

`api_keys`：

```text
id, organization_id, project_id, name, environment, key_prefix, key_hash,
scopes_json, model_policy_json, ip_allowlist_json, rate_limit_policy_id,
expires_at, last_used_at, revoked_at, created_by, created_at
```

`usage_events`：

```text
id, request_id, organization_id, project_id, api_key_id,
external_model, resolved_model, channel_id, source_id,
input_tokens, output_tokens, cached_tokens, modality_units,
provider_cost_micro_usd, customer_charge_micro_usd,
pricing_version_id, status, started_at, completed_at
```

`ledger_entries` 和 `ledger_postings` 必须形成不可变双分录。任何余额展示均从账本投影而来，
可使用物化余额表加速，但投影表不得成为唯一真相。

### 8.3 索引、分区与保留期

- 所有多租户表以 `(organization_id, created_at)` 或 `(project_id, created_at)` 建复合索引。
- `request_logs`、`usage_events`、`audit_logs` 按月分区，防止长期索引膨胀。
- 使用量明细保留至少 18 个月；安全审计保留至少 24 个月；法律或合同要求更高时从其规定。
- 默认不持久化完整 prompt/response。若客户开启调试保留，必须独立加密、单租户隔离、
  明确 TTL，并提供删除入口。
- 所有外键、唯一键和状态转换约束在数据库层实现，不能仅依赖应用校验。

---

## 9. Redis、事件与异步任务

### 9.1 Redis 可用于

- 固定窗口/滑动窗口/令牌桶限流。
- API Key、项目、渠道的并发计数。
- 熔断状态、回退退避、健康检查短期结果。
- 精确缓存、幂等请求短期结果、会话撤销列表。
- 分布式锁，但锁必须设置过期与持有者令牌校验。

### 9.2 Redis 不可用于

- 唯一账本、余额真相、支付最终状态、渠道凭证、用户长期权限。
- 唯一请求审计或唯一异步消息来源。

### 9.3 异步任务

采用 PostgreSQL Transactional Outbox：业务事务提交时，同步写入 `outbox_events`；
Worker 可靠投递到 NATS JetStream 或直接消费。任务类型至少包括：

- 支付到账、退款、对账、余额告警。
- 渠道健康检查、凭证验证、Token 自动刷新（subscription pool OAuth）、成本同步、路由缓存失效。
- 用量聚合、日报、异常毛利告警。
- Webhook 投递、失败重试、死信处理。
- 邮件、MFA 通知、工单通知、数据归档。

每个任务必须有：事件 ID、幂等键、重试次数、最大尝试次数、退避策略、死信状态、
可观测 trace ID。消费者必须幂等。

---

## 10. 后台与客户门户

### 10.1 后台信息架构

后台不使用长页面堆砌表格，应采用固定侧边栏、清晰二级页、筛选持久化和移动端降级布局。
所有危险动作必须在抽屉或独立确认页完成，禁止多个浮层相互遮挡。

| 一级模块 | 必须页面 |
| --- | --- |
| 数据概览 | 收入、成本、毛利、请求成功率、P95 延迟、余额告警、渠道健康 |
| 用户与组织 | 用户、组织、成员、项目、风险状态、支持历史 |
| API 与模型 | API Key、套餐、限流、模型目录、模型映射、能力矩阵 |
| 渠道管理 | Provider、账号来源、渠道、路由规则、健康检查、成本与预算 |
| 计费财务 | 钱包、账本、充值、退款、价格版本、支付配置、对账差异 |
| 运营安全 | 审计日志、安全事件、IP 规则、Feature Flag、Webhook、系统设置 |
| 客服工单 | 工单队列、消息、关联请求摘要、退款/异常处理流 |

### 10.2 渠道管理页面细节

账号来源页必须支持：

- 新建、编辑元数据、验证、禁用、轮换凭证、查看脱敏健康状态。
- 批量启停和批量标签，但删除需逐条确认且默认软删除。
- 仅显示凭证指纹、最后验证时间、错误分类；永不回显明文。
- 以卡片和表格双视图显示来源，卡片显示剩余预算、并发、成功率、最近异常。
- 订阅池视图：显示池内账号总数、按状态分布（active / cooldown / exhausted /
  dead / manual_intervention）、总剩余配额估算、Arkose 打码成功率和成本。
  支持按账号标签、状态、剩余配额、最后错误码筛选。
- 导入订阅账号：
  - **手动粘贴**：批量粘贴 Session Token / Refresh Token / Access Token 列表
    （每行一个），系统自动检测类型并进入对应刷新管道。
  - **HAR 上传**：上传浏览器 DevTools 导出的 HAR 文件，平台内存解析提取所有
    Token，加密后即时删除原始文件。
  - **Cookie JSON**：粘贴浏览器导出的 Cookie JSON，平台解析并进入 Cookie 保活管道。
  - 批量导入时显示每行的解析状态（成功/类型检测/格式错误/需人工）。
- Arkose / CAPTCHA 配置：选择 Solver 供应商（YesCaptcha / CapSolver / 2Captcha），
  填入 API Key（加密存储），设置超时、最大重试次数、单次成本上限。支持 Solver
  供应商冗余（主 + 备自动切换）。实时显示打码成功率和单次成本。
- 代理绑定：每账号配置 HTTP / SOCKS5 / 住宅代理端点（加密存储），选择轮换策略
  （static / per_request / per_session）。代理健康状态实时监控。
- 单账号操作：触发立即刷新、查看刷新历史、查看打码历史、手动冷却/恢复、
  立即禁用、标记失效、重配代理。
- 路由模拟器：输入组织、项目、模型、区域，输出候选排序和”为何未选中”解释；
  对 subscription_pool 来源展示池内账号的选中/未选中原因及健康状态；
  该工具仅管理员可用，结果不含秘密。

### 10.3 客户门户页面

- 首页：余额、今日用量、成功率、最近错误、项目预算。
- 项目：新建项目、模型权限、环境变量说明。
- API Key：创建、显示一次、轮换、禁用、IP 白名单、使用范围。
- 用量：按日期、模型、项目、状态、Key 筛选；提供 CSV 导出与时区选择。
- 账单：余额、充值、扣费、发票/收据状态、价格版本说明。
- 开发者：快速开始、代码示例、模型能力表、状态页、错误码。
- 支持：提交工单、查看处理进度、关联 request ID。

### 10.4 前端安全与交互规范

- 前端只承担展示与提交，不承担权限判断、价格计算、余额校验或密钥加密。
- 仅使用同源 `/admin-api` 和 `/portal-api`；不得在编译产物中出现源站域名或秘密。
- 表格支持服务端分页、筛选、排序，不一次性加载全量使用日志。
- 每个保存动作返回明确状态；异步操作显示可追踪任务 ID。
- 页面必须处理加载、空态、权限不足、网络失败、过期会话和移动端窄屏。
- 所有上传文件进行 MIME、大小、恶意内容和对象存储下载签名控制。

---

## 11. 安全设计

### 11.1 网络与边缘

- API 和后台均启用 Cloudflare 代理、TLS 1.2+、HSTS、WAF、DDoS 防护和 Bot/速率规则。
- 源站通过 Cloudflare Tunnel 主动出站连接，不对公网开放应用端口。
- VPS 防火墙仅允许受限 SSH；SSH 采用新建运维用户、密钥登录、禁止 root 密码远程登录。
- 后台域名接入 Cloudflare Access，限制管理员身份、MFA、可选 IP/设备策略。
- API CORS 默认关闭；若需浏览器 SDK，仅允许精确 Origin 白名单，禁止 `*` 与凭证并用。

### 11.2 应用安全

- 所有输入按 JSON Schema 验证，限制 Body、数组、嵌套、Header、文件和 SSE 生命周期。
- 上游请求使用域名允许列表、固定端口、禁用私网 IP 解析与重定向，防 SSRF。
- 使用参数化 SQL，禁用动态拼接 SQL；上传文件使用随机对象名且禁止直接执行。
- 响应安全头至少包含 CSP、`X-Content-Type-Options`、`Referrer-Policy`、
  `Permissions-Policy`、`frame-ancestors 'none'`。
- 所有管理修改带 `audit_log`，包含操作者、时间、对象、前后摘要、IP、trace ID、原因。
- 审计日志采用追加写；每日可输出带 hash chain 的归档到受控对象存储。

### 11.3 密钥与加密

- 渠道密钥、OAuth Refresh Token、支付密钥使用 Envelope Encryption。
- 数据密钥使用 AES-256-GCM；主密钥由云 KMS/Vault 或至少独立部署密钥管理服务保护。
- 密钥密文携带 `key_version`，支持无停机轮换。
- 生产密钥不可写入 `.env` 以外的开发文件、Git、截图、工单、报错或监控标签。
- 日志系统在入口处脱敏 `Authorization`、Cookie、支付签名、请求正文敏感字段。

### 11.4 反滥用与合规

- 对 API Key、组织、IP、模型分别限流，监测异常并发、突增消耗、失败率和地域跳变。
- 支持项目硬预算、软预算告警、单日上限、模型级上限和紧急冻结开关。
- 维护服务条款、隐私政策、可接受使用政策、退款政策和数据保留策略。
- 客户请求仍受每个上游厂商政策约束。平台不能承诺绕过供应商限制。
- 若业务涉及转售，必须先确认每个上游的正式转售/代理/企业协议与税务要求。

---

## 12. 可观测性、告警与 SLO

### 12.1 必须采集的指标

- 请求量、成功率、4xx/5xx、P50/P95/P99 延迟、TTFB、SSE 活跃连接数。
- 按模型、渠道、Provider、组织、项目的成功率、超时、回退率、熔断状态。
- Token、图像/音频单位、收入、上游成本、毛利、缓存命中率、预占释放率。
- Redis 命中率、内存、连接数；PostgreSQL 连接池、慢查询、锁、复制/备份状态。
- 支付回调失败、账本不平、对账差异、Webhook 积压、死信队列长度。

指标标签不得包含 API Key、用户邮箱、完整 request ID、prompt、响应正文等高基数或敏感值。

### 12.2 日志与追踪

- 每个入口生成 `request_id` 与 OpenTelemetry trace ID。
- 日志采用结构化 JSON，级别为 `debug/info/warn/error`。
- 生产默认记录元数据而非 prompt/response；用户可控调试日志有独立权限和 TTL。
- Trace 必须跨越边缘、数据面、适配器、异步任务和控制面，但不得携带授权头或正文。

### 12.3 SLO 与告警

初始 SLO：

| 指标 | 目标 |
| --- | --- |
| API 月可用性 | 99.9%（计划维护除外） |
| 非流式网关内部开销 P95 | 小于 150 ms，不含上游模型生成时间 |
| 流式首字节网关开销 P95 | 小于 200 ms，不含上游首字节时间 |
| 支付入账延迟 P95 | 小于 5 分钟 |
| 账本日终平衡 | 100% |

告警至少覆盖：渠道全失效、成本突增、余额异常、错误率、延迟、队列积压、备份失败、
证书/Tunnel 故障、管理员异常登录和审计日志写入失败。

---

## 13. 部署、备份与灾难恢复

### 13.1 环境

- `development`：本地 Compose，模拟 Provider，使用虚拟支付。
- `staging`：独立数据库、Redis、Cloudflare 测试域名、真实但低额度官方 API。
- `production`：独立密钥、独立数据库、正式域名、严格网络策略。

开发、测试、生产之间不得复用数据库、Redis、支付商户号、上游渠道凭证或 JWT/KMS 主密钥。

### 13.2 生产部署基线

- 所有服务容器化，镜像使用不可变 Git SHA 标签。
- Docker Compose 适用于第一阶段单机；服务必须有健康检查、资源限制、重启策略和只读文件系统。
- PostgreSQL 与 Redis 优先使用托管服务或至少独立于应用容器的持久卷；数据库不暴露公网。
- Cloudflare Tunnel 运行两个连接器实例；单服务器时至少运行两个进程，后续迁移到第二节点。
- 数据面滚动升级前需要停止接收新请求、等待 SSE 连接排空或在超时后优雅中断。
- 数据库迁移必须先备份、先在 staging 演练、只允许向前兼容；破坏性迁移拆分为多版本。

### 13.3 备份策略

- 每次部署、数据库迁移、路由/价格批量变更前创建可恢复备份并记录位置。
- PostgreSQL：每日全量备份、持续 WAL 归档、至少 30 天保留、每月执行恢复演练。
- 对象存储：开启版本化和跨区域/跨账户备份，备份必须加密。
- Redis 不作为唯一数据源；关键限流/缓存丢失应自动恢复。
- Git：每个功能、修复、配置变更单独提交并推送 GitHub；禁止 force push 主分支。
- 备份包、数据库 dump、`.env` 和生产密钥不得提交到 Git。

### 13.4 灾难恢复目标

| 系统 | RPO | RTO |
| --- | --- | --- |
| 账本与支付 | 5 分钟以内 | 4 小时以内 |
| 控制面 | 15 分钟以内 | 4 小时以内 |
| 数据面 | 可接受无状态重建 | 1 小时以内 |
| 使用日志 | 15 分钟以内 | 24 小时以内 |

必须编写并演练：数据库恢复、单渠道故障、Redis 清空、密钥泄漏、支付重复回调、
Cloudflare 故障、服务器丢失和误删组织数据的 Runbook。

### 13.5 单机容量与扩容结论

当前单台服务器可以作为早期验证环境，但不能承诺支撑“完全体”高可用生产：

- API 网关主要受并发流式连接、网络带宽、上游延迟、PostgreSQL IOPS 和日志量影响，
  不能只按 CPU 核数估算。
- 建议生产起步至少使用 4 vCPU、8 GB RAM、快速 SSD、独立 PostgreSQL 备份和外部 Redis；
  真实容量必须用 k6 压测验证。
- 达到单机资源 60% 持续负载、SSE 连接接近上限、数据库 P95 延迟上升或可用性目标受影响时，
  先拆分 PostgreSQL/Redis，再横向扩展数据面，最后考虑多区域。

---

## 14. 测试与质量门禁

### 14.1 必测层级

- 单元测试：账本计算、限流、路由排序、熔断、价格、权限、密钥哈希、错误映射。
- 契约测试：OpenAPI、OpenAI/Anthropic/Gemini 兼容响应、SSE 事件顺序、Provider Adapter。
- 集成测试：PostgreSQL、Redis、Outbox、支付回调、对象存储、加密轮换。
- E2E 测试：注册、MFA、Key 创建/轮换、充值、调用、用量、退款、工单、后台渠道管理。
- 性能测试：非流式 QPS、SSE 并发、长连接、缓存命中、限流、降级与回退。
- 安全测试：SSRF、SQL 注入、越权、IDOR、CSRF、XSS、CORS、密钥泄漏、Webhook 重放。
- 灾难测试：渠道全失效、Redis 不可用、数据库只读、重复回调、Worker 重启、网络分区。

### 14.2 CI/CD 门禁

每个 Pull Request 必须通过：

- 格式化、静态检查、类型检查、单元测试、契约测试。
- 依赖漏洞扫描、Secret 扫描、SAST、容器镜像扫描、SBOM 生成。
- 数据库迁移向前兼容检查、OpenAPI 破坏性变更检查。
- 前端可访问性基础检查、关键 E2E 流程。

主分支合并后构建不可变镜像并部署 staging。生产部署需要变更单、备份记录、
健康检查和回滚/前滚方案。财务、密钥、支付和权限模块必须至少一人代码审查。

---

## 15. 分期实施与验收

### 阶段 0：项目基线与安全底座

交付：Monorepo、环境模板、Compose、配置校验、CI、Secret 扫描、OpenAPI 骨架、
Cloudflare Worker 骨架、PostgreSQL/Redis 连接、日志/追踪、备份 Runbook。

验收：任何服务不含生产秘密；一键拉起 development；CI 可构建；源站不对公网暴露。

### 阶段 1：身份、组织、项目、密钥

交付：用户登录、MFA、组织/项目、RBAC、API Key 创建/轮换/禁用、审计日志、客户门户基础页。

验收：Key 仅展示一次；跨组织访问被拒绝；后台高危操作有审计；MFA 未启用的管理员不可进入敏感页。

### 阶段 2：OpenAI 主协议数据面

交付：`/v1/models`、`/v1/chat/completions`、`/v1/responses`、`/v1/embeddings`、
SSE、统一错误、请求日志、限流、并发限制、幂等键。

验收：SDK 可调用；SSE 正常取消；不暴露源站或渠道；超额请求正确拒绝；接口契约测试通过。

### 阶段 3：渠道、模型与智能路由

交付：官方 API/OAuth/上游来源管理、Provider Adapter、模型目录、模型映射、健康检查、
优先级/权重/预算/熔断/安全回退、后台路由模拟器。

验收：自有官方来源优先；故障时按规则回退；无候选时返回明确错误；渠道秘密不回显。

### 阶段 4：计费、支付、账本与用量

交付：价格版本、余额预占/结算、双分录账本、充值订单、支付宝/微信聚合支付接口、
回调幂等、用量报表、对账任务和异常队列。

验收：重复回调不重复入账；账本平衡；失败请求正确释放；历史价格不可篡改；对账可定位差异。

### 阶段 5：多厂商兼容与开发者体验

交付：Anthropic/Gemini 兼容层、图像/音频/审核、SDK 示例、开发者文档、Webhook、
精确缓存、状态页、客户工单。

验收：能力矩阵准确；不支持字段明确报错；缓存不跨租户；文档示例可在 staging 跑通。

### 阶段 6：生产强化与规模化

交付：SSO、Cloudflare Access、分级审批、异常风控、灾备演练、压测报告、双节点数据面、
自动化备份恢复、成本/毛利异常告警。

验收：达到 SLO；完成恢复演练；关键安全测试无高危问题；可在不丢账的前提下扩容。

---

## 16. 开发团队必须先产出的设计件

编码前必须提交并评审以下文件：

1. `PRD.md`：用户、套餐、模型范围、支付和客服流程。
2. `ERD.md`：全部表、关系、索引、RLS 与分区方案。
3. `openapi/`：数据面、控制面、门户 API 的 OpenAPI 3.1 文档。
4. `ADR/`：技术栈、加密、路由、账本、缓存、多厂商转换、Cloudflare 架构决策。
5. `THREAT_MODEL.md`：资产、信任边界、攻击路径、缓解措施、剩余风险。
6. `PROVIDER_CAPABILITY_MATRIX.md`：每个模型/厂商的输入、输出、工具、视觉、流式、
   成本、区域、限制和测试状态。
7. `RUNBOOKS/`：部署、回滚、数据库恢复、渠道故障、支付差异、密钥泄漏。
8. `TEST_PLAN.md`：本文件第 14 章全部验收项的自动化与人工测试映射。

未完成上述设计件，不允许直接开始支付、账本、渠道凭证或路由核心代码。

---

## 17. 最终上线检查清单

- 所有生产域名走 Cloudflare，真实源站无公共应用端口。
- 前端、API、控制面域名与路由明确隔离，浏览器无源站泄漏。
- 所有管理角色已启用 MFA，紧急 Owner 账户受独立保护。
- API Key、渠道凭证和支付密钥已完成加密、脱敏、轮换和泄漏扫描验证。
- 账本双分录、支付回调幂等、退款、日对账已在 staging 演练。
- 自有官方 API 来源优先、上游回退、熔断、预算和路由模拟器已验收。
- `subscription_pool` 类型来源已完整实现：支持 session_token / refresh_token /
  access_token / cookie_jar / har_archive 五种凭证类型导入；OAuth 自动刷新
  + Cookie 保活已就绪；HAR 解析管道加密即焚已验证；Arkose/CAPTCHA 打码
  （YesCaptcha/CapSolver/2Captcha）集成并测试通过；每账号代理绑定和健康
  监控已就绪；配额追踪/冷却/故障分级升级/告警已验收。无浏览器引擎内置、
  无自动注册逻辑。
- 备份、WAL 归档、恢复演练、监控告警、值班联系人和事故 Runbook 已就绪。
- OpenAPI、SDK 示例、错误码、模型能力、价格说明、服务条款与隐私政策已发布。
- 压测结果证明实际并发、带宽、数据库和上游配额符合目标 SLO。

满足以上条件后，Token Gateway 才可作为面向客户收费的生产平台上线。
