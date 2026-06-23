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
- 上游渠道池负责自有 API Key 池、企业项目 Key 池、合规代理上游兜底。
- Redis/PostgreSQL 接口预留，MVP 可先单机，后续必须拆分。

## 重要边界

网页登录 Plus/Pro 账号池属于高风险能力。该项目可以记录需求和风险评估，但不得实现绕过网页登录风控、逆向 Web 接口、Cookie 池共享、自动化规避验证码或共享订阅权益的逻辑。

如需做“用户自有账号接入”，必须走合规授权模式：官方 API Key、OAuth、服务账号、企业项目凭证或用户显式授权的官方接口。
