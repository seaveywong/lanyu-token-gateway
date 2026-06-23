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

Plus/Pro 订阅账号池通过 `subscription_pool` 来源类型支持。Token 由运营人员一次性导入，
平台通过标准 OAuth 2.0 流程自动化维持（后台 Cron 定时刷新）。参考实现：New API (25k+ Stars)、
Sub2API (21k+ Stars)、chat2api (18k+ Stars)。

明确不做：
- 网页登录自动化（用户名+密码填表）
- 验证码/MFA 绕过、CAPTCHA 打码服务集成
- Cookie Jar 自动化维护、浏览器自动化（Selenium/Puppeteer/Playwright）
- 逆向 ChatGPT/Claude/Gemini 网页 WebSocket 协议

详见 `docs/RISK_BOUNDARIES.md`。
