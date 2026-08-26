# 文档索引

欢迎使用 herald-smtp 文档。herald-smtp 是 [Herald](https://github.com/soulteary/herald) 的 SMTP 邮件适配器。

## 多语言文档

- [English](../enUS/README.md) | [中文](README.md)

## 文档列表

### 核心文档

- **[README.md](../../README.md)** - 项目概览与快速开始
- **[CHANGELOG.md](../../CHANGELOG.md)** - 发布历史与 v1 兼容性基线

### 详细文档

- **[API.md](API.md)** - 完整 API 参考
  - Base URL 与认证
  - POST /v1/send 字段行为与可运行请求示例
  - GET /healthz 的存活检查语义
  - GET /readyz 的本地 SMTP 就绪语义
  - 错误码与 HTTP 状态
  - 幂等、SMTP 并发与 `message_id` 行为

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - 部署指南
  - 二进制与 Docker 运行
  - 完整配置与 SMTP TLS 模式矩阵
  - 健康检查、关闭行为与副本限制
  - 与 Herald 集成（HERALD_SMTP_API_URL）

- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - 故障排查
  - 收不到邮件
  - 503 provider_down
  - 401 unauthorized
  - HTTP 状态快速参考
  - invalid_destination、send_failed、幂等与日志

- **[SECURITY.md](SECURITY.md)** - 安全实践
  - API Key 使用
  - SMTP 凭证管理
  - 请求元数据与日志暴露
  - 生产建议

## 快速导航

### 入门

1. 阅读 [README.md](../../README.md) 了解项目
2. 查看 [快速开始](../../README.md#快速开始)
3. 参考 [DEPLOYMENT.md](DEPLOYMENT.md) 进行配置与 Herald 集成

### 开发

1. 查看 [API.md](API.md) 了解发送契约与错误码
2. 参考 [DEPLOYMENT.md](DEPLOYMENT.md) 了解部署方式

### 运维

1. 阅读 [DEPLOYMENT.md](DEPLOYMENT.md) 了解部署与 Herald 侧配置
2. 参考 [SECURITY.md](SECURITY.md) 了解生产实践
3. 排障请参考 [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
