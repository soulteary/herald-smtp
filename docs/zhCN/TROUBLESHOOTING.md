# herald-smtp 故障排查指南

本指南帮助诊断和解决 herald-smtp 的常见问题。

## 目录

- [收不到邮件](#收不到邮件)
- [503 provider_down](#503-provider_down)
- [401 Unauthorized](#401-unauthorized)
- [invalid_destination](#invalid_destination)
- [send_failed](#send_failed)
- [HTTP 状态快速参考](#http-状态快速参考)
- [幂等与日志](#幂等与日志)

## HTTP 状态快速参考

| 状态 | 含义 | 首先检查 |
|-----:|------|----------|
| 400 | JSON 无效、收件人缺失、地址或邮件头不合法 | 请求体和收件地址 |
| 401 | API Key 缺失或不匹配 | `API_KEY` 和 `X-API-Key` |
| 409 | 同一幂等 key 被用于不同请求内容 | 调用方生成和复用 key 的逻辑 |
| 413 | 请求超过 `HTTP_BODY_LIMIT_BYTES` | 请求体大小和配置上限 |
| 429 | Provider 限流或幂等存储容量不足 | Provider 配额和 `IDEMPOTENCY_MAX_ENTRIES` |
| 503 | SMTP 未配置或无法初始化 | 启动日志、`SMTP_HOST`、`SMTP_FROM` 和 TLS 模式 |
| 504 | 发送或幂等等待超过截止时间 | SMTP 延迟和超时配置 |
| 500 | SMTP 发送失败或出现意外内部错误 | 服务端日志；HTTP 错误信息会刻意保持通用 |

`GET /healthz` 只是存活检查。返回 200 不能排除 SMTP 配置、认证、网络连通性或最终投递问题。

## 收不到邮件

### 现象

- Herald 创建了 channel 为 `email` 的 challenge 并从 herald-smtp 得到成功响应，但用户未收到邮件。

### 排查步骤

1. **查看 herald-smtp 日志**  
   查找 `send_failed` 或 SMTP 错误：
   ```bash
   # Docker 运行时
   docker logs herald-smtp 2>&1 | grep -E "send_failed|send ok"
   ```
   - `send ok` 且带 `message_id`：SMTP 服务器已经接受该邮件，但最终投递仍可能因下游拒绝、过滤、垃圾邮件判定或地址错误而失败。
   - `send_failed` 且带 errmsg：记录错误信息用于下一步。

2. **确认 SMTP 配置**  
   - 确认已设置 `SMTP_HOST`、`SMTP_FROM`；若服务器需要认证，设置 `SMTP_USER` 和 `SMTP_PASSWORD`。
   - 使用 `openssl s_client -starttls smtp -connect SMTP_HOST:587` 测试 STARTTLS，或使用 `openssl s_client -connect SMTP_HOST:465` 测试隐式 TLS；请替换真实主机名。

3. **检查收件人与垃圾邮件**  
   - 确认 `to`（destination）为有效邮箱且无拼写错误。
   - 检查收件人垃圾邮件/垃圾箱。

### 处理

- **凭证错误**：更新 `SMTP_HOST`、`SMTP_USER`、`SMTP_PASSWORD`、`SMTP_FROM` 并重启 herald-smtp。
- **地址错误或无效**：确保 Herald 为 channel `email` 传入有效的邮箱作为 `destination`。
- **SMTP 限流**：检查 SMTP 服务商是否有限流或封禁。

---

## 503 provider_down

### 现象

- `POST /v1/send` 返回 HTTP 503，body：`"ok": false, "error_code": "provider_down", "error_message": "SMTP not configured"`。

### 原因

herald-smtp 要求 `SMTP_HOST` 和 `SMTP_FROM` 非空。任一缺失则不会初始化 SMTP 客户端，每次发送都返回 503。

### 处理

1. 设置 `SMTP_HOST` 和 `SMTP_FROM`（若需要认证则一并设置），并重启进程（或容器）。
2. 确认运行时中确实存在这些变量（如环境变量名无拼写错误，Docker/Kubernetes 中正确传入）。
3. 查看启动日志：若凭证缺失，herald-smtp 会打印警告，说明 `/v1/send` 将返回 503。

---

## 401 Unauthorized

### 现象

- `POST /v1/send` 返回 HTTP 401，`error_code: "unauthorized"`, `error_message: "invalid or missing API key"`。

### 原因

herald-smtp 配置了 `API_KEY`，但请求未携带 `X-API-Key` 或携带的值不匹配。

### 处理

1. **若需要 API Key 认证**  
   - 在 herald-smtp 上设置 `API_KEY`。  
   - 在 Herald 上将 `HERALD_SMTP_API_KEY` 设为相同值，以便 Herald 在 `X-API-Key` 中发送。  
   - 确认代理或网关未剥离 `X-API-Key` 头。

2. **若不需要 API Key**  
   - 在 herald-smtp 上不设置 `API_KEY`（Herald 上也不设置 `HERALD_SMTP_API_KEY`）。

---

## invalid_destination

### 现象

- `POST /v1/send` 返回 HTTP 400，`error_code: "invalid_destination"`, `error_message: "to is required"`。

### 原因

请求体中 `to` 字段为空或缺失。

### 处理

1. 确保 Herald 为 channel `email` 传入非空的 `to`（收件人邮箱）。
2. 确认从用户标识到邮箱的映射正确且不会产生空字符串。

---

## send_failed

### 现象

- `POST /v1/send` 返回 HTTP 500，`error_code: "send_failed"`，外部错误信息为通用描述。

### 原因

SMTP 发送因连接、认证、TLS 或服务器拒绝而失败。详细传输错误只记录在服务端日志中，不会返回给调用方。

### 处理

1. 确认 `SMTP_HOST`、`SMTP_PORT`、`SMTP_USER`、`SMTP_PASSWORD`、`SMTP_USE_STARTTLS` 与 SMTP 服务商一致（如 587 端口 + STARTTLS，或 465 + TLS）。
2. 检查 herald-smtp 到 SMTP 服务器的网络连通性（防火墙、DNS）。
3. 确认 SMTP 服务器允许发件地址（`SMTP_FROM`）且凭证正确。
4. 在服务端日志中查找 `send_failed: SMTP error`；HTTP 响应不会包含内部主机或连接细节。

---

## 幂等与日志

### 幂等命中（缓存响应）

`Idempotency-Key`（或 body 中的 `idempotency_key`）最长 256 字节。使用相同 key 和内容的并发请求只触发一次 SMTP 发送，其余请求等待并复用成功结果；在配置的 TTL 内再次提交相同请求时，herald-smtp 会直接返回缓存响应。失败请求可继续重试；若同一 key 对应不同请求内容，则返回 `409 idempotency_conflict`。

如果增加副本后才出现重复发送，应检查请求路由。幂等状态仅保存在当前进程，多个副本不共享 key 或缓存结果；横向扩容前应保持单副本或增加共享幂等层。

### 日志级别

- **info**：可看到 `send ok`、`send_failed` 以及上述 503/401。
- **debug**：还可看到 `send idempotent hit`。将 `LOG_LEVEL=debug` 可验证相同幂等 key 的重复请求是否被缓存。

不要使用秘密或个人数据作为幂等 key；调用方提供的 key 可能作为 `message_id` 返回并写入日志。

### TTL

幂等缓存 TTL 由 `IDEMPOTENCY_TTL_SECONDS`（默认 300）控制。超过 TTL 后，相同 key 会被视为新请求并可能触发新的发送。

如果新 key 返回 `429 rate_limited`，表示内存存储已达到 `IDEMPOTENCY_MAX_ENTRIES`（默认 10000）。已有 key 仍可使用；可减少唯一 key 的产生速度、等待条目过期，或在服务内存预算内提高上限。
