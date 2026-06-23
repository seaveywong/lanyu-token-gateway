# AI 对接操作规划

## 给 AI 开发代理的固定约束

1. 每次修改前必须创建备份。
2. 每次修改后必须提交 Git，并推送到 GitHub。
3. 不允许提交密钥、Cookie、网页登录 Session、数据库文件。
4. API 中转项目独立于商城，不得把中转网关逻辑塞回 `D:\dev\FB`。
5. 对外统一 `api.lanyu.one/v1/*`。
6. 预留 Redis/PostgreSQL，不把 SQLite 作为长期 API 网关账本。

## MVP 目标

```text
客户 API Key
套餐余额
模型映射
渠道池（含订阅账号池 subscription_pool）
自有官方 API/OAuth 优先
自有订阅池（Plus/Pro/Team）次之
合规上游代理兜底
Token 自动刷新（后台 Cron，标准 OAuth）
限速
用量日志
基础倍率
精确响应缓存
```

## 推荐模块

```text
src/http          HTTP 服务入口
src/auth          客户 API Key 鉴权
src/router        模型路由和渠道选择
src/providers     Provider Adapter
src/billing       余额扣减与账本
src/cache         Redis/内存缓存适配
src/storage       PostgreSQL/SQLite 适配
src/admin         后台管理 API
src/audit         审计日志
```

## Provider Adapter 规范

每个上游适配器必须实现：

```text
listModels()
chatCompletions(request)
responses(request)
embeddings(request)
normalizeError(error)
estimateUsage(request, response)
validateCredential(credential)    -- 包括 OAuth Token 验证
refreshToken(refreshToken)        -- OAuth refresh_token 换 access_token
```

## 渠道选择策略

```text
1. 查模型映射
2. 查客户余额和限速
3. 选择支持模型的渠道
4. 自有官方 API/OAuth 优先
5. 自有订阅池次之（池内按最少负载 + 加权轮询选账号）
6. 同优先级按权重轮询
7. 合规上游代理兜底
8. 失败后自动切换
9. 连续失败进入冷却
10. 成功后写用量和扣费账本
```

## 缓存策略

第一版只做：

```text
模型映射缓存
渠道健康缓存
客户余额/限速缓存
精确响应缓存
```

不做语义缓存，避免隐私和答非所问风险。
