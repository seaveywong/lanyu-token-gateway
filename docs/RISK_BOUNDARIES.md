# 风险边界：订阅账号池 (subscription_pool)

## 设计背景

本平台 Plus/Pro/Team 订阅账号池的设计**直接对齐开源社区成熟方案**：

| 项目 | Stars | 核心自动化能力 |
|---|---|---|
| **New API** (QuantumNous) | 25k+ | OAuth 自动刷新 + Arkose Token 配置 + 渠道健康检测 + 余额自动探测 + 多机主从部署 |
| **Sub2API** (Wei-Shaw) | 21k+ | TokenRefreshService goroutine + 最少负载调度 + 粘性会话 + 双层并发槽 + OAuth/API Key/Cookie 三种凭证类型 |
| **chat2api** (lanqian528) | 18k+ | refresh_token → access_token 链式刷新 + YesCaptcha/CapSolver Arkose 打码 + HAR 文件导入 + 多账号池轮询 |
| **chatgpt2api** (zgm2003) | 5k+ | 号池管理 + CPA/sub2api 号池导入 + 注册机维持号池额度 + 配额到期自动冷却 |
| **cto-new-openai-proxy** | 1k+ | LRU 账号自动轮换 + Clerk JWT 自动刷新 + Playwright 批量注册 + Dashboard 实时监控 |

合计 **70k+ Stars** 的生产级验证。本平台在开源方案基础上增加了企业级加密、
审计和故障隔离，未削减任何开源社区已验证的自动化能力。

## 支持的自动化能力

| 能力 | 说明 | 参考实现 |
|---|---|---|
| OAuth Token 自动刷新 | 后台 Cron 检测 Token 有效期，过期前通过 `/oauth/token` 自动刷新 | New API, Sub2API, chat2api |
| Session Token → OAuth 交换 | 导入 `session_token` 后自动完成 OAuth 交换，进入长期刷新循环 | New API, Sub2API |
| Cookie 保活 | 对不支持 OAuth 的 Provider，通过定期 HTTP 请求维持 Cookie 有效性 | Sub2API, chat2api |
| HAR 文件解析导入 | 解析浏览器 HAR 导出，自动提取所有可用 Token 并分发到对应刷新管道 | chat2api, chatgpt2api |
| Arkose/CAPTCHA 自动打码 | 集成 YesCaptcha / CapSolver / 2Captcha，OAuth 端点触发验证时自动完成 | chat2api, New API（可选配置）|
| 代理绑定 | 每个账号可绑定 HTTP/SOCKS5/住宅代理，降低风控检测 | New API 社区实践 |
| 配额追踪与自动冷却 | 账号配额耗尽后标记 `exhausted`，窗口到期后自动恢复 | chatgpt2api, cto-new-openai-proxy |
| 故障分级升级 | 按连续失败次数自动升级：active → cooldown → dead → 告警 | New API, Sub2API |
| 批量导入 | 换行分隔 Token 列表、HAR 归档文件、Cookie JSON | chatgpt2api (批量导入 CPA 号池) |

## 明确不做的（唯一红线）

| 禁止项 | 原因 |
|---|---|
| 自动注册账号 | 涉及虚假身份、支付欺诈，法律风险不可接受 |
| 信用卡/支付自动化 | 同上 |
| 浏览器 UI 自动化提取 Token（Selenium / Puppeteer / Playwright 内置于平台） | Token 提取由运营人员在**自己的浏览器**中完成（DevTools / HAR 导出），平台不内置浏览器引擎 |

除此之外，**不做额外限制**。开源社区跑通的能力，本平台全部纳入。

## 运营风险与缓解

| 风险 | 概率 | 影响 | 缓解措施 |
|---|---|---|---|
| 账号被封 | 中 | 单账号失效 | 池化冗余 + 自动 dead 标记 + 告警通知补号。单个账号死亡不影响池内其他账号 |
| Arkose 难度升级 | 中 | 打码成功率下降 | 多 Solver 供应商冗余 + 自动切换 + `manual_intervention` 兜底 |
| Token 端点限流 | 低 | 刷新延迟 | 分散刷新窗口 + 随机抖动 + 代理 IP 轮换 |
| IP 信誉降级 | 中 | 请求被拦截 | 住宅代理绑定 + 每账号独立 IP + 信誉评分监控 |
| 供应商 ToS 变更 | 低 | 渠道整体不可用 | 多供应商冗余 + 官方 API 兜底 + 合规上游回退 |
| HAR 文件残留 | 低 | 凭证泄漏 | 内存解析、即时加密、原始文件不落盘 |

## 安全设计（对齐开源最佳实践 + 企业增强）

### 凭证加密

- AES-256-GCM 信封加密，主密钥由 KMS 或独立部署的密钥管理服务保护。
- 与 New API / Sub2API 的加密级别对齐（两者均使用 AES 加密渠道凭证）。
- 额外增强：支持密钥版本化（`key_version`），无停机密钥轮换。

### 审计与告警

- 所有 Token 刷新、CAPTCHA 打码、账号状态变更均记录审计日志。
- 审计日志不含明文 Token、完整 OAuth 响应体、Solver API Key。
- 告警覆盖：Token 刷新成功率 < 90%、池可用比例 < 30%、单账号连续失败 ≥ 6 次、
  账号收到 401/403（疑似封号）、打码成功率 < 70%。

### 网络隔离

- Token 刷新请求仅通过允许列表中的出口主机发出。
- CAPTCHA Solver API 调用同样受限。
- 代理流量经独立网络接口（若配置）。

## 合规说明

本平台是一个**技术工具**，账号池功能的使用方式由运营方自行决定。平台提供方
不鼓励、不协助用户违反任何第三方服务条款。

建议运营方：
1. 了解上游供应商的服务条款。
2. 在客户协议中明确服务可能使用的技术路径。
3. 为 Plus/Pro 账号购买商业订阅（非个人订阅），降低合规风险。
4. 保持官方 API 渠道作为主要供给，订阅池作为补充/回退。

开源社区（New API、Sub2API、chat2api 等合计 70k+ Stars）已在该模式下
运行数年，未出现大规模法律诉讼案例。
