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

检查服务健康状态，由 [health-kit](https://github.com/soulteary/health-kit) 实现。

**成功响应：**
```json
{
  "status": "healthy",
  "service": "herald-smtp"
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
| `channel` | string | 否 | Herald 调用时通常为 `"email"`。 |
| `to` | string | 是 | 收件人邮箱地址。 |
| `subject` | string | 否 | 邮件主题。为空时默认 "Verification code"。 |
| `body` | string | 否 | 邮件正文。为空时见下方内容解析。 |
| `idempotency_key` | string | 否 | 幂等键；TTL 内相同 key 返回缓存结果。 |
| `template` | string | 否 | 可选；当前实现未使用。 |
| `params` | object | 否 | 若 `body` 为空且存在 `params.code`，正文为 "Your verification code is: " + params.code。 |
| `locale` | string | 否 | 可选。 |

**内容解析顺序：**
1. 若 `body` 非空，使用 `body`。
2. 否则若存在 `params.code`，使用 "Your verification code is: " + params.code。
3. 否则使用默认："You have a verification message. Please check your code."

**成功响应 – HTTP 200：**
```json
{
  "ok": true,
  "message_id": "uuid-or-challenge-id",
  "provider": "smtp"
}
```

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
| `invalid_request` | 400 | 请求体解析失败（无效 JSON）。 |
| `invalid_destination` | 400 | `to` 缺失或为空。 |
| `provider_down` | 503 | 未配置 SMTP（SMTP_HOST / SMTP_FROM 未设置）。 |
| `send_failed` | 500 | SMTP 发送错误（连接、认证或服务器错误）。 |

## 幂等

- 发送请求支持通过 `Idempotency-Key` 头或 body 字段 `idempotency_key` 实现幂等。
- 在配置的 TTL（`IDEMPOTENCY_TTL_SECONDS`，默认 300）内，相同 key 的重复请求返回缓存响应（相同 `ok`、`message_id`、`provider`），不再重复发送。
- 缓存为内存；key 在 TTL 后过期。
