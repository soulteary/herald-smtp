# herald-smtp 安全实践

本文描述 herald-smtp 的安全考虑与建议。

## API Key

- 当配置 `API_KEY` 时，herald-smtp 要求 **POST /v1/send** 的 `X-API-Key` 头与之匹配。请使用强且唯一的值并保密。
- Herald 需将 `HERALD_SMTP_API_KEY` 配置为相同值，以便在每次请求 herald-smtp 时发送该 key。
- 不要记录或暴露 API key。优先使用环境变量或密钥管理服务，而非提交到版本库的配置文件。

## SMTP 凭证

- **SMTP_HOST**、**SMTP_USER**、**SMTP_PASSWORD**、**SMTP_FROM** 不得硬编码或提交到仓库。
- 将其存放在环境变量或密钥管理服务（如 Kubernetes Secrets、HashiCorp Vault）中。仅将 `.env` 用于本地开发，并确保 `.env` 在 `.gitignore` 中。
- 定期轮换 SMTP 密码并更新 herald-smtp 配置。

## 生产建议

- **网络**：在私有网络中运行 herald-smtp。仅 Herald（或你的网关）应调用它；除非在 HTTPS 与严格访问控制之后，否则不要将 herald-smtp 直接暴露到公网。
- **HTTPS**：若 herald-smtp 在互联网或不可信网络中可访问，应置于带 TLS 的反向代理（如 Traefik、nginx）之后。此时 Herald 的 `HERALD_SMTP_API_URL` 应使用 `https://`。
- **最小权限**：使用非 root 用户运行进程；在 Docker 中尽可能使用非 root 用户镜像。
- **日志**：避免记录可能包含敏感信息的请求体或请求头。结构化日志（如脱敏的 `to`、`message_id`、错误码）足以满足运维与排障。

## 小结

- 在生产环境使用 `API_KEY` 并保密；在 Herald 中配置 `HERALD_SMTP_API_KEY` 与之一致。
- 将 SMTP 凭证存放在环境变量或密钥管理服务中；切勿放在代码或提交的配置中。
- 优先使用私有网络并在 herald-smtp 前使用 HTTPS；无保护情况下不要公网暴露。
