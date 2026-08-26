# herald-smtp 部署指南

## 快速开始

### 二进制

```bash
# 构建
go build -o herald-smtp .

# 运行（先设置 SMTP 环境变量）
./herald-smtp
```

### Docker

```bash
# 拉取固定版本
docker pull ghcr.io/soulteary/herald-smtp:v1.0.0

# 运行并传入环境变量
docker run -d --name herald-smtp -p 8084:8084 --stop-timeout=40 \
  --health-cmd='curl -fsS http://localhost:8084/healthz || exit 1' \
  --health-interval=30s --health-timeout=3s \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_FROM=noreply@example.com \
  -e SMTP_USER=user \
  -e SMTP_PASSWORD=secret \
  ghcr.io/soulteary/herald-smtp:v1.0.0
```

如需构建当前源码，可运行 `docker build -t herald-smtp:local .`，并将示例中的镜像名替换为 `herald-smtp:local`。生产环境应固定使用具体版本标签，而不是 `latest`。

### Docker Compose 示例

```yaml
services:
  herald-smtp:
    image: ghcr.io/soulteary/herald-smtp:v1.0.0
    stop_grace_period: 40s
    ports:
      - "8084:8084"
    environment:
      - PORT=:8084
      - SMTP_HOST=${SMTP_HOST}
      - SMTP_FROM=${SMTP_FROM}
      - SMTP_USER=${SMTP_USER}
      - SMTP_PASSWORD=${SMTP_PASSWORD}
      # 可选：
      # - API_KEY=${API_KEY}
      # - LOG_LEVEL=info
      # - SMTP_MAX_CONCURRENT_SENDS=16
      # - IDEMPOTENCY_TTL_SECONDS=300
      # - SHUTDOWN_TIMEOUT_SECONDS=40
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8084/healthz"]
      interval: 30s
      timeout: 3s
      retries: 3
```

可选：若在 herald-smtp 上配置了 `API_KEY`，传入该值并在 Herald 侧将 `HERALD_SMTP_API_KEY` 设为相同值：

```bash
docker run -d --name herald-smtp -p 8084:8084 --stop-timeout=40 \
  -e API_KEY=your_shared_secret \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_FROM=noreply@example.com \
  -e SMTP_USER=user \
  -e SMTP_PASSWORD=secret \
  ghcr.io/soulteary/herald-smtp:v1.0.0
```

## 配置

### 环境变量

| 变量 | 说明 | 默认值 | 必填 |
|------|------|--------|------|
| `PORT` | 监听端口（可带或不带冒号，如 `8084` 或 `:8084`） | `:8084` | 否 |
| `API_KEY` | 若设置，调用方需在 `X-API-Key` 中传此值 | `` | 否 |
| `SMTP_HOST` | SMTP 服务器主机 | `` | 是（发送时） |
| `SMTP_PORT` | SMTP 端口 | `587` | 否 |
| `SMTP_USER` | SMTP 用户名 | `` | 否（若服务器允许匿名） |
| `SMTP_PASSWORD` | SMTP 密码 | `` | 否 |
| `SMTP_FROM` | 发件人邮箱地址 | `` | 是（发送时） |
| `SMTP_FROM_NAME` | 可选的发件人显示名称 | `` | 否 |
| `SMTP_USE_TLS` | 使用隐式 TLS（通常为 465 端口） | `false` | 否 |
| `SMTP_USE_STARTTLS` | 使用 STARTTLS | `true` | 否 |
| `SMTP_SKIP_TLS_VERIFY` | 跳过证书校验，仅限开发环境 | `false` | 否 |
| `SMTP_TIMEOUT_SECONDS` | SMTP 完整发送过程的超时时间 | `30` | 否 |
| `SMTP_MAX_CONCURRENT_SENDS` | 同时进行的 SMTP 发送数量上限 | `16` | 否 |
| `LOG_LEVEL` | 日志级别：trace, debug, info, warn, error | `info` | 否 |
| `IDEMPOTENCY_TTL_SECONDS` | 幂等缓存 TTL（秒） | `300` | 否 |
| `IDEMPOTENCY_MAX_ENTRIES` | 处理中及已缓存幂等键的最大总数 | `10000` | 否 |
| `HTTP_BODY_LIMIT_BYTES` | HTTP 请求体大小上限 | `65536` | 否 |
| `HTTP_READ_TIMEOUT_SECONDS` | HTTP 请求读取超时 | `10` | 否 |
| `HTTP_WRITE_TIMEOUT_SECONDS` | HTTP 响应写入超时 | `40` | 否 |
| `HTTP_IDLE_TIMEOUT_SECONDS` | HTTP 长连接空闲超时 | `60` | 否 |
| `SHUTDOWN_TIMEOUT_SECONDS` | 优雅关闭最长等待时间，实际值不会短于 SMTP 超时加 5 秒 | `40` | 否 |

`SMTP_USE_TLS` 与 `SMTP_USE_STARTTLS` 不能同时启用。使用隐式 TLS 时，应设置 `SMTP_USE_TLS=true` 和 `SMTP_USE_STARTTLS=false`。
实际 HTTP 写入超时始终不低于 `SMTP_TIMEOUT_SECONDS + 5` 秒，确保 SMTP 操作能在响应截止时间前完成。

当 `SMTP_HOST` 或 `SMTP_FROM` 缺失时，`POST /v1/send` 返回 `503`，`error_code` 为 `"provider_down"`。

`SMTP_MAX_CONCURRENT_SENDS` 是非阻塞容量限制。所有发送槽位均被占用时，新请求会立即返回 `429 rate_limited`，不会进入无界等待队列。应结合 SMTP 服务商的连接限制以及容器的 CPU、内存和文件描述符预算进行调整。

### SMTP 传输模式

| 模式 | 常用端口 | `SMTP_USE_TLS` | `SMTP_USE_STARTTLS` | 说明 |
|------|---------:|:--------------:|:-------------------:|------|
| STARTTLS | 587 | `false` | `true` | 默认模式；SMTP 服务支持时优先使用。 |
| 隐式 TLS | 465 | `true` | `false` | 建立连接后立即开始 TLS。 |
| 明文 SMTP | 25 或自定义 | `false` | `false` | 仅用于可信私有网络。 |

除隔离的开发环境外，应保持 `SMTP_SKIP_TLS_VERIFY=false`。

## 健康检查与关闭

- `/healthz` 是**存活检查**，仅确认 HTTP 进程能够响应，不检查 SMTP 配置、认证、网络连通性或最终投递。
- `/readyz` 是**就绪检查**，仅在 SMTP Client 已根据有效本地配置完成初始化时返回成功；它不会测试 SMTP 网络连通性或最终投递。
- 收到 `SIGINT` 或 `SIGTERM` 后，服务停止接收新请求，并按 `SHUTDOWN_TIMEOUT_SECONDS` 等待 HTTP 关闭。实际关闭超时不会短于 `SMTP_TIMEOUT_SECONDS + 5` 秒，以便进行中的发送能够完成。
- 容器运行时或进程管理器在发送 `SIGKILL` 前的等待时间必须不少于实际关闭超时。示例将两者都设为 40 秒；若实际关闭超时更长，应同步增大 `--stop-timeout` 或 `stop_grace_period`。

## 副本与幂等模型

幂等预留和缓存结果保存在进程内存中，不会在容器或主机之间共享。如果同一 key 到达不同副本，每个副本都可能各自执行 SMTP 发送。需要依赖本地幂等时应运行单副本；需要横向扩容时，应由调用方或网关提供共享幂等层。

## 与 Herald 集成

当 OTP 通道为 `email` 且 Herald 配置了 `HERALD_SMTP_API_URL` 时，Herald 通过 HTTP 调用 herald-smtp。在 Herald 中配置：

- **`HERALD_SMTP_API_URL`** – herald-smtp 的 Base URL（例如 `http://herald-smtp:8084`）。
- **`HERALD_SMTP_API_KEY`**（可选） – 与 herald-smtp 的 `API_KEY` 相同；Herald 会将其放在 `X-API-Key` 中发送。

设置 `HERALD_SMTP_API_URL` 后，Herald 不再使用内置 SMTP（Herald 中无需配置 `SMTP_HOST`）。所有 SMTP 凭证仅存在于 herald-smtp。

Herald 对同一次逻辑发送的重试应复用稳定的幂等 key，不得将同一 key 用于不同收件人或不同内容。
