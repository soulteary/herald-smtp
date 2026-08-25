# herald-smtp 安全实践

本文描述 herald-smtp 的安全考虑与建议。

## API Key

- 当配置 `API_KEY` 时，herald-smtp 要求 **POST /v1/send** 的 `X-API-Key` 头与之匹配。请使用强且唯一的值并保密。
- Herald 需将 `HERALD_SMTP_API_KEY` 配置为相同值，以便在每次请求 herald-smtp 时发送该 key。
- 不要记录或暴露 API key。优先使用环境变量或密钥管理服务，而非提交到版本库的配置文件。
- 确保反向代理和访问日志对 `X-API-Key` 进行脱敏。

## 请求元数据

- 将幂等 key 视为非秘密的关联标识。调用方提供的 key 可能作为 `message_id` 返回并写入日志；不得在其中放入密码、访问 Token、邮箱地址或其他敏感信息。
- 错误响应会刻意避免暴露意外的 SMTP 主机、网络或连接细节；排障应使用受保护的服务端日志。

## SMTP 凭证

- **SMTP_HOST**、**SMTP_USER**、**SMTP_PASSWORD**、**SMTP_FROM** 不得硬编码或提交到仓库。
- 生产环境应保持 `SMTP_SKIP_TLS_VERIFY=false`。跳过证书校验会使 SMTP 连接面临中间人攻击。
- 将其存放在环境变量或密钥管理服务（如 Kubernetes Secrets、HashiCorp Vault）中。仅将 `.env` 用于本地开发，并确保 `.env` 在 `.gitignore` 中。
- 定期轮换 SMTP 密码并更新 herald-smtp 配置。

## 生产建议

- **网络**：在私有网络中运行 herald-smtp。仅 Herald（或你的网关）应调用它；除非在 HTTPS 与严格访问控制之后，否则不要将 herald-smtp 直接暴露到公网。
- **HTTPS**：若 herald-smtp 在互联网或不可信网络中可访问，应置于带 TLS 的反向代理（如 Traefik、nginx）之后。此时 Herald 的 `HERALD_SMTP_API_URL` 应使用 `https://`。
- **最小权限**：使用非 root 用户运行进程；项目提供的 Docker 镜像使用专用的 `herald` 用户。
- **资源上限**：保持请求大小、HTTP 超时和幂等存储容量限制有效；应按实际流量调整，不要通过设置过大的值变相关闭限制。
- **副本模型**：内存幂等状态仅属于当前进程；若要在多个实例之间防止重复发送，需要增加共享幂等层。
- **日志**：避免记录可能包含敏感信息的请求体或请求头。结构化日志（如脱敏的 `to`、`message_id`、错误码）足以满足运维与排障。

## 小结

- 在生产环境使用 `API_KEY` 并保密；在 Herald 中配置 `HERALD_SMTP_API_KEY` 与之一致。
- 将 SMTP 凭证存放在环境变量或密钥管理服务中；切勿放在代码或提交的配置中。
- 优先使用私有网络并在 herald-smtp 前使用 HTTPS；无保护情况下不要公网暴露。
