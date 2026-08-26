# herald-smtp API 文档

herald-smtp 实现 Herald 外部 Provider 在 email 通道使用的 HTTP 发送协议，请求/响应类型与 [provider-kit](https://github.com/soulteary/provider-kit) 的 `HTTPSendRequest` / `HTTPSendResponse` 一致。

## Base URL

```
http://localhost:8084
```

## 认证

当配置了 `API_KEY` 时，Herald（或任意调用方）必须在请求头中携带 `X-API-Key`，且值与 herald-smtp 的 `API_KEY` 一致。若未携带或不一致，返回 `401 Unauthorized`，`error_code` 为 `"unauthorized"`。

未配置 `API_KEY` 时，`/v1/send` 不需要认证。

## 端点

### 健康检查

**GET /healthz**

检查 HTTP 进程是否存活，由 [health-kit](https://github.com/soulteary/health-kit) 实现。此端点不会验证 SMTP 配置、凭证、DNS、网络连通性或最终投递；即使 `/v1/send` 返回 `503 provider_down`，它仍可能返回 HTTP 200。

**成功响应：**
```json
{
  "status": "healthy",
  "service": "herald-smtp"
}
```

### 就绪检查

**GET /readyz**

SMTP Client 初始化成功时返回 HTTP `200` 和 `status: ready`；SMTP 配置缺失或无效时返回 HTTP `503` 和 `status: not_ready`。该检查只验证本地初始化状态，不会连接 SMTP 服务器，也不代表邮件最终投递成功。

**就绪响应 – HTTP 200：**
```json
{
  "status": "ready",
  "service": "herald-smtp"
}
```

**未就绪响应 – HTTP 503：**
```json
{
  "status": "not_ready",
  "service": "herald-smtp",
  "reason": "smtp_not_configured"
}
```

### 发送（SMTP 邮件）

**POST /v1/send**

通过 SMTP 发送邮件。当 Herald 配置了 `HERALD_SMTP_API_URL` 且 channel 为 `email` 时由 Herald 调用。

**请求头：**
- `X-API-Key`（可选）：当 herald-smtp 配置了 `API_KEY` 时必传且需一致。
- `Idempotency-Key`（可选）：用于幂等发送；也可在请求体中通过 `idempotency_key` 设置。
- `Content-Type`：`application/json`

**请求体（HTTPSendRequest）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `channel` | string | 否 | 为兼容 Provider 协议而接收；当前处理器不校验该字段。Herald 通常传入 `"email"`。 |
| `to` | string | 是 | 收件人邮箱地址。 |
| `subject` | string | 否 | 邮件主题。为空时默认 "Verification code"。 |
| `body` | string | 否 | 邮件正文。为空时见下方内容解析。 |
| `idempotency_key` | string | 否 | 幂等键，最长 256 字节；TTL 内相同 key 和请求内容返回缓存结果。 |
| `template` | string | 否 | 为兼容协议而接收；当前实现不会据此选择或渲染模板。 |
| `params` | object | 否 | 会传递给 Provider Message；若 `body` 为空且存在 `params.code`，还会用于生成正文。 |
| `locale` | string | 否 | 会传递给 Provider Message；当前 SMTP 内容生成不会按 locale 本地化。 |

**内容解析顺序：**
1. 若 `body` 非空，使用 `body`。
2. 否则若存在 `params.code`，使用 "Your verification code is: " + params.code。
3. 否则使用默认："You have a verification message. Please check your code."

**请求示例：**

```bash
curl -sS -X POST http://localhost:8084/v1/send \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: replace-with-your-api-key' \
  -H 'Idempotency-Key: challenge-123' \
  -d '{"channel":"email","to":"recipient@example.com","params":{"code":"123456"}}'
```

仅当服务端未配置 `API_KEY` 时才应省略 `X-API-Key`。

**成功响应 – HTTP 200：**
```json
{
  "ok": true,
  "message_id": "uuid-or-challenge-id",
  "provider": "smtp"
}
```

未提供幂等键时，`message_id` 由 SMTP Provider 生成；提供幂等键时，provider-kit v1.5.0 会将该 key 作为 `message_id` 返回，缓存响应也返回相同值。应将其视为不透明的关联标识，而不是邮件最终投递成功的证明。

**失败响应：**
```json
{
  "ok": false,
  "error_code": "error_code",
  "error_message": "可读说明"
}
```

**错误码与 HTTP 状态：**

| error_code | HTTP 状态 | 说明 |
|------------|-----------|------|
| `unauthorized` | 401 | 已配置 `API_KEY` 但未传或错误的 `X-API-Key`。 |
| `invalid_request` | 400 或 413 | 请求解析失败、幂等 key 冲突或过长，或请求体超过配置上限。 |
| `invalid_destination` | 400 | `to` 缺失或为空。 |
| `validation_failed` | 400 | 收件地址或邮件头校验失败。 |
| `idempotency_conflict` | 409 | 幂等键与已有请求冲突。 |
| `rate_limited` | 429 | SMTP 并发容量用尽、上游服务触发限流，或幂等存储达到容量上限。 |
| `provider_down` | 503 | 未配置 SMTP（SMTP_HOST / SMTP_FROM 未设置）。 |
| `timeout` | 504 | SMTP 发送超过截止时间。 |
| `send_failed` | 500 | SMTP 发送错误（连接、认证或服务器错误）。 |
| `not_found` | 404 | 没有与请求路径匹配的路由。 |
| `method_not_allowed` | 405 | 路由不支持当前 HTTP 方法。 |

超过 `HTTP_BODY_LIMIT_BYTES` 的请求体会在 JSON 解析前由 HTTP 服务器以 `413` 状态拒绝。经由 Fiber Handler 返回的路由不存在、方法不支持和请求实体过大等错误使用与 `/v1/send` 相同的 JSON 响应结构。Fasthttp 的底层请求体限制可能在进入 Fiber Handler 前直接拒绝请求。

## 幂等

- 发送请求支持通过 `Idempotency-Key` 头或 body 字段 `idempotency_key` 实现幂等。
- 在配置的 TTL（`IDEMPOTENCY_TTL_SECONDS`，默认 300）内，相同 key 的成功请求会返回缓存响应（相同 `ok`、`message_id`、`provider`），不再重复发送。
- 使用相同 key 和内容的并发请求只触发一次 SMTP 发送，其余请求等待并复用首个成功结果。
- 失败发送不会缓存，因此临时 SMTP 故障可使用同一 key 重试。
- 缓存为内存；key 在 TTL 后过期。
- 存储容量由 `IDEMPOTENCY_MAX_ENTRIES` 限制（默认 10000）。全部槽位占用时，新 key 返回 `429 rate_limited`，已有 key 仍可继续使用。
- 状态仅存在于单个进程。多个副本之间不共享预留或缓存，同一 key 被路由到不同实例时可能触发多次 SMTP 发送。
- 不要在幂等 key 中放入密码、Token、邮箱地址或其他秘密。该 key 可能作为 `message_id` 返回并进入结构化日志。

## SMTP 并发

`SMTP_MAX_CONCURRENT_SENDS` 限制同时调用 SMTP Provider 的数量（默认 16）。限制仅在当前进程内生效，并且不会阻塞等待：所有槽位均被占用时，新请求会立即收到 `429 rate_limited`。服务不会建立无界 SMTP 发送队列。
