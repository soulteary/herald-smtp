# herald-smtp 故障排查指南

本指南帮助诊断和解决 herald-smtp 的常见问题。

## 目录

- [收不到邮件](#收不到邮件)
- [503 provider_down](#503-provider_down)
- [401 Unauthorized](#401-unauthorized)
- [invalid_destination](#invalid_destination)
- [send_failed](#send_failed)
- [幂等与日志](#幂等与日志)

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
   - `send ok` 且带 `message_id`：herald-smtp 已通过 SMTP 成功发送；投递问题可能在 SMTP 服务器或收件方（垃圾邮件、地址错误）。
   - `send_failed` 且带 errmsg：记录错误信息用于下一步。

2. **确认 SMTP 配置**  
   - 确认已设置 `SMTP_HOST`、`SMTP_FROM`；若服务器需要认证，设置 `SMTP_USER` 和 `SMTP_PASSWORD`。
   - 测试 SMTP 连通性（如 `telnet SMTP_HOST 587`），若 `SMTP_USE_STARTTLS=true` 则确认 STARTTLS。

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

- `POST /v1/send` 返回 HTTP 500，`error_code: "send_failed"`，`error_message` 包含 SMTP 或网络错误详情。

### 原因

SMTP 发送失败：连接被拒、认证失败、TLS 错误或服务器拒绝。

### 处理

1. 确认 `SMTP_HOST`、`SMTP_PORT`、`SMTP_USER`、`SMTP_PASSWORD`、`SMTP_USE_STARTTLS` 与 SMTP 服务商一致（如 587 端口 + STARTTLS，或 465 + TLS）。
2. 检查 herald-smtp 到 SMTP 服务器的网络连通性（防火墙、DNS）。
3. 确认 SMTP 服务器允许发件地址（`SMTP_FROM`）且凭证正确。

---

## 幂等与日志

### 幂等命中（缓存响应）

当 Herald（或任意客户端）在配置的 TTL 内使用相同的 `Idempotency-Key`（或 body 中的 `idempotency_key`）发送请求时，herald-smtp 会返回缓存响应而不再次发送。这是预期行为。

### 日志级别

- **info**：可看到 `send ok`、`send_failed` 以及上述 503/401。
- **debug**：还可看到 `send idempotent hit`。将 `LOG_LEVEL=debug` 可验证相同幂等 key 的重复请求是否被缓存。

### TTL

幂等缓存 TTL 由 `IDEMPOTENCY_TTL_SECONDS`（默认 300）控制。超过 TTL 后，相同 key 会被视为新请求并可能触发新的发送。
