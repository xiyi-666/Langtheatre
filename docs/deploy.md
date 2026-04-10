# LinguaQuest MVP 部署手册

## 1. 前置条件

- GitHub 仓库已配置 Actions secrets。
- Cloudflare（Workers + R2）已开通。
- 一台可运行 Go 服务的云主机（GCP/AWS）。

## 2. 依赖服务

本地或云上通过 `infra/docker-compose.yml` 启动：

- Redis 7
- RabbitMQ 3.13
- PostgreSQL 使用 Supabase 托管实例（`SUPABASE_DB_URL`）

## 3. API 服务部署

1. 在 `apps/server` 执行 `go build -o linguaquest-api ./cmd/server`
2. 上传二进制与环境变量至云主机
3. 使用 systemd 或 supervisor 托管进程
4. 对外暴露 `:8080` 并配置 HTTPS
5. 确保部署环境具备 IPv6 访问能力（Supabase 直连要求）

## 4. Cloudflare Edge 代理

1. 修改 `infra/cloudflare/wrangler.toml` 的 `ORIGIN_API`
2. 在该目录执行 `wrangler deploy`
3. 客户端 `VITE_API_URL` 指向 Worker 域名

## 5. R2 媒体存储

1. 创建 `linguaquest-media` bucket
2. 配置 Access Key 到后端环境变量
3. 开启 CDN 加速（可绑定自定义域名）

## 6. Sentry 监控

1. 前端和后端分别创建项目
2. 填充 `SENTRY_DSN` 与 `VITE_SENTRY_DSN`
3. 验证错误上报（手工触发一次异常）

## 7. 部署后烟雾与性能检查

1. 访问 `GET /healthz` 与 `GET /readyz`，确认依赖可用。
2. 执行关键 GraphQL 冒烟：`register/login/me/generateTheater/submitAnswers`。
3. 可选执行性能基线：`npm run test:perf`（需要安装 k6）。
