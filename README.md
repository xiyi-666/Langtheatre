# LinguaQuest

简体中文 | [English](README.en.md)

跨平台语言学习应用，集成剧场式对话、阅读训练与角色扮演，提供完整练习闭环（播放 → 答题 → 结算 → XP）。

Quick Start • 项目介绍 • 技术架构 • 环境配置 • 快速启动 • 部署与文档

## 📝 项目介绍

LinguaQuest 使用 Tauri 2 + React + TypeScript + Go 构建，面向英语与粤语等语言练习场景，支持“剧场生成、角色扮演、阅读训练、成绩结算与成长体系”。

核心目标：用剧情化、可复盘的对话形式降低语言学习门槛，并提供可追踪的学习成果。

## ✨ 关键能力

- 剧场对话：生成多轮对话，支持播放、收藏与分享码
- 角色扮演：多回合对话评估、即时反馈与总结
- 阅读训练：分段阅读、答题与学习词汇/语法
- 练习闭环：答题结算、XP 累计、个人中心进度
- 工程能力：自动迁移、健康探针、基础限流与请求追踪

## 🧭 技术架构

```
┌─────────────────────────────┐
│        Tauri Client          │
│  React + TS + Vite + Tauri   │
└───────────────┬─────────────┘
                │ GraphQL
┌───────────────▼─────────────┐
│          Go Server           │
│   GraphQL API + 服务层       │
└───────┬─────────┬───────────┘
        │         │
  PostgreSQL     Redis / MQ
 (Supabase)    (Cache / Queue)
```

## 🧱 技术栈

- 前端：React、TypeScript、Vite、Tauri 2
- 后端：Go 1.22、GraphQL、PostgreSQL（Supabase）、Redis、RabbitMQ
- 基础设施：Docker / Docker Compose、Cloudflare Workers + R2
- 测试：Go 单测、前端单测、Playwright E2E、k6 性能基线

## 📁 目录结构

- apps/client: Tauri 2 客户端（Windows/macOS/Android）
- apps/server: Go GraphQL 服务
- infra: 本地依赖与部署脚本
- docs: 技术与运维文档
- tests/e2e: 前端关键路径 E2E

## ⚙️ 环境配置

后端环境变量示例见：docs/env.example.md

常用配置（apps/server/.env）：

```
PORT=8080
JWT_SECRET=dev-secret-change-me
REDIS_ADDR=localhost:6379
SUPABASE_DB_URL=postgresql://postgres:YOUR_PASSWORD_URLENCODED@db.xxx.supabase.co:5432/postgres
OPENAI_API_KEY=
OPENAI_MODEL=gpt-4o-mini
OPENAI_BASE_URL=https://api.openai.com
TTS_API_URL=
TTS_API_KEY=
TTS_VOICE=female-1
```

前端环境变量示例：

```
VITE_API_URL=http://localhost:8080/graphql
```

## 🚀 快速启动

### 方式一：本地开发

1) 安装依赖

```
# 项目根目录
npm install

# 后端依赖
cd apps/server
go mod tidy
```

2) 启动基础依赖（Redis / RabbitMQ）

```
docker compose -f infra/docker-compose.yml up -d redis rabbitmq
```

3) 启动后端

```
cd apps/server
go run ./cmd/server
```

4) 启动前端

```
# 项目根目录
npm run client:dev
```

5) 健康检查

```
GET http://localhost:8080/healthz
GET http://localhost:8080/readyz
```

### 方式二：桌面端（Tauri 2）

```
# 开发
npm run tauri:dev --workspace apps/client

# 构建
npm run tauri:build --workspace apps/client
```

## 📦 部署

- 部署指南：docs/deploy.md
- 启动与排障：docs/startup.md
- API 说明：docs/api.md

## ✅ 测试矩阵

- 后端单测：cd apps/server && go test ./...
- 前端单测：npm run test --workspace apps/client
- 前端 E2E：npm run test --workspace tests/e2e
- 性能基线：npm run test:perf（需安装 k6）

## 📚 文档导航

- 启动文档：docs/startup.md
- 部署手册：docs/deploy.md
- 环境变量：docs/env.example.md
- API 文档：docs/api.md

## 📜 License

本项目使用仓库内 LICENSE 文件的条款。

## 🖼️ 产品截图

| 登录页 | 主页 | 生成页 |
| --- | --- | --- |
| ![登录页](docs/images/%E7%99%BB%E5%BD%95%E9%A1%B5.png) | ![主页](docs/images/%E5%AF%B9%E8%AF%9D%E9%A1%B5.png) | ![生成页](docs/images/%E7%94%9F%E6%88%90%E9%A1%B5.png) |
