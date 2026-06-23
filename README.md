# 蓝域 API 中转站 / Token Gateway

本项目独立于 `D:\dev\FB` 商城项目，负责 API 中转、客户 API Key、余额扣减、渠道池、模型映射、缓存、限速、日志和风控。

## 目标域名

```text
https://api.lanyu.one
```

## 对外接口目标

优先提供 OpenAI-compatible 接口：

```text
GET  /v1/models
POST /v1/chat/completions
POST /v1/responses
POST /v1/embeddings
```

内部通过 Provider Adapter 兼容：

- OpenAI
- Azure OpenAI
- Anthropic Claude
- Google Gemini / Vertex AI
- OpenRouter
- DeepSeek
- DashScope / Qwen
- Moonshot / Kimi
- 智谱 GLM
- 火山方舟
- SiliconFlow
- Mistral / Groq / Together
- 任意 OpenAI-compatible 上游

## 架构原则

- 商城负责售卖、订单、套餐、客户余额来源。
- Token Gateway 负责鉴权、限速、路由、扣费、日志。
- 上游渠道池负责自有 API Key 池、企业项目 Key 池、订阅账号池（Plus/Pro/Team Session/Refresh Token，通过标准 OAuth 自动刷新）和合规代理上游兜底。
- Redis/PostgreSQL 接口预留，MVP 可先单机，后续必须拆分。

## 重要边界

Plus/Pro 订阅账号池通过 `subscription_pool` 来源类型完整支持，能力直接对齐
开源社区成熟方案（New API 25k+ / Sub2API 21k+ / chat2api 18k+ Stars）：

- OAuth Token 自动刷新（后台 Cron）
- Session Token → OAuth 交换
- Cookie 保活
- HAR 文件解析导入
- Arkose/CAPTCHA 自动打码（YesCaptcha / CapSolver / 2Captcha）
- 每账号代理绑定（HTTP / SOCKS5 / 住宅 IP）
- 配额追踪、冷却、故障分级升级

唯一红线：不做自动注册账号、不做支付欺诈、平台不内置浏览器引擎。

详见 `docs/RISK_BOUNDARIES.md`。
