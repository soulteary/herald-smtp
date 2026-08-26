# Security

## Supported Versions

Starting with v1.0.0, security fixes are provided for the current `1.x` release line. Pre-1.0 releases are no longer supported.

| Version | Supported |
|---------|-----------|
| `1.x` | Yes |
| `<1.0` | No |

Security practices for herald-smtp are documented in the docs:

- **[English](docs/enUS/SECURITY.md)** – API Key usage, SMTP credential management, production recommendations
- **[中文](docs/zhCN/SECURITY.md)** – API Key 使用、SMTP 凭证管理、生产环境建议

**Summary**: Use `API_KEY` in production and keep it secret; store SMTP credentials in environment variables or a secret manager (never in code or committed config); prefer private network and HTTPS in front of herald-smtp.

To report a security vulnerability, please open a private security advisory or contact the maintainers directly.
