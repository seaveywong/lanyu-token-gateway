# 风险边界：订阅账号池 (subscription_pool)

## 设计背景

本平台支持 Plus/Pro/Team 等订阅账号作为 API 渠道来源（`subscription_pool` 类型）。
该设计参考了开源社区主流实践（New API、Sub2API、chat2api 等，合计 60k+ Stars），
但在此之上增加了更严格的安全与合规约束。

## 允许的操作

- 运营人员通过后台管理界面**一次性导入**已提取的 Session Token 或 Refresh Token。
  Token 来源可以是浏览器 DevTools、官方 OAuth 回调或其他运营方认可的方式。
- 平台通过**标准 OAuth 2.0 流程**自动完成 Token 交换（session_token → access_token +
  refresh_token）和持续刷新（refresh_token → new access_token）。
- 平台按账号粒度跟踪：配额消耗、Token 过期时间、冷却时间和健康状态。
- 账号不可用时（配额耗尽 / 冷却 / Token 失效 / 连续故障），路由引擎自动跳过。
- 客户在路由模拟器和审计日志中可感知请求是否路由到 `subscription_pool` 来源。

## 安全要求（MUST）

| 要求 | 说明 |
|---|---|
| Envelope Encryption | Token 使用 AES-256-GCM 信封加密，与官方 API Key 同等级保护。主密钥由 KMS 或独立部署的密钥管理服务保护 |
| 不存储身份凭据 | 不存储用户名、密码、Cookie Jar、浏览器指纹、用户代理串 |
| 不做浏览器自动化 | 不集成 Selenium / Puppeteer / Playwright 或任何浏览器自动化工具 |
| 不做 CAPTCHA 绕过 | 不集成任何打码服务（YesCaptcha / CapSolver / 2Captcha 等）。若 OAuth 端点要求 Arkose/CAPTCHA，账号标记为 `manual_intervention`，通知运营人员手动处理 |
| 不做逆向工程 | 不逆向 ChatGPT / Claude / Gemini 网页 WebSocket 协议、API 端点签名算法或请求加密方案 |
| 白名单出口 | Token 刷新请求仅允许发往预先配置的 OAuth 端点（如 `auth.openai.com`、`auth.anthropic.com`），禁止任意 URL |
| 审计追踪 | 每个订阅账号标注其来源供应商、购买凭证编号，供审计。Token 刷新、账号状态变更、路由决策均记录审计日志 |
| 故障隔离 | 账号 `dead` 后立即从路由中移除，需人工确认后重新激活 |
| 客户透明 | 路由至订阅池的请求在审计日志中记录 `source_type=subscription_pool`，但不记录具体账号标签 |
| 立即禁用 | 收到 401/403 或账号被封通知后，立即标记 `dead` 并告警 |

## 仍然禁止的操作

以下操作无论何种形式均不得实现：

- 模拟网页登录（用户名+密码自动化填表、自动点击登录按钮）。
- 自动绕过验证码、二次验证（MFA）、设备验证或任何形式的反机器人挑战。
- Cookie Jar 自动化维护（自动续期 Cookie、自动切换浏览器 Profile、自动清理缓存）。
- 逆向 ChatGPT/Claude/Gemini 网页接口（WebSocket 协议分析、请求签名逆向、
  响应解密、gizmo/gizmo_id/user-agent 伪造等不属于标准 OAuth 流程的操作）。
- 将个人 Plus/Pro 订阅包装后转售给第三方，除非供应商服务条款明确允许。
- 自动化注册账号、自动化订阅购买、自动化额度重置利用。

## 开源社区对比

本平台的设计借鉴了以下开源项目的实践经验，但在自动化程度上做了收敛：

| 项目 | Stars | 自动化方式 | 本平台对应策略 |
|---|---|---|---|
| **New API** | 25k+ | 后台 Cron 自动刷 OAuth Token；支持 Arkose Token 配置（手动获取后填入） | 采用相同模型：后台 Cron 自动刷标准 OAuth Token。**不做** Arkose 自动化——仅通知运营手动提供 |
| **Sub2API** | 21k+ | `TokenRefreshService` goroutine 过期前自动刷新；刷新期间标记 unschedulable | 采用相同模型：后台 goroutine 提前刷新 + 刷新期间跳过 |
| **chat2api** | 18k+ | refresh_token → access_token 链式刷新 + Arkose 打码服务集成 | 采用链式刷新模型。**去掉** Arkose 打码服务集成 |
| **chatgpt2api** | 5k+ | 注册机维持号池额度 + 自动刷新 | 采用号池 + 自动刷新。**去掉**注册机（不自动注册账号） |

## 合规评估要求

在启用 `subscription_pool` 类型之前，必须完成以下评估并生成 ADR：

1. 每个供应商的服务条款中关于 API 访问、Token 共享、转售的条款摘要。
2. 供应商 OAuth 端点是否公开可用、是否有速率限制、是否支持 `refresh_token` grant。
3. 账号被封禁的历史概率和触发条件分析。
4. 数据保护影响评估（DPIA）：客户数据是否经过非官方 API 链路。
5. 客户告知义务：是否需要在服务条款或隐私政策中披露"部分请求可能路由至订阅账号池"。

评估结果为"高风险"的供应商，仅可在内部测试环境使用，不得作为生产级渠道来源。

## 告警与监控

- Token 刷新成功率低于 90% 时触发 P2 告警。
- 池内可用账号比例低于 30% 时触发 P2 告警。
- 单个账号连续刷新失败 ≥ 6 次时触发 P3 告警（账号 dead）。
- 池内所有账号 dead → 来源自动熔断 → P1 告警。
- 任何账号收到 401/403 → 即时 P2 告警（疑似封号）。

指标采集：`token_refresh_success_rate`、`pool_available_account_ratio`、
`account_status_transitions_total`、`account_quota_exhaustion_total`。
