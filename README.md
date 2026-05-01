# LinguaQuest

[English](README.en.md) | 简体中文

一个跨平台语言学习应用，结合剧场式对话、阅读练习和角色扮演，形成完整练习闭环（听 → 作答 → 结果 → XP）。

快速开始 • 概览 • 架构 • 配置 • 上手指南 • 部署与文档

## 📝 概览

LinguaQuest 基于 Tauri 2 + React + TypeScript + Go 构建，面向英语与粤语练习场景，支持剧场生成、角色扮演、阅读练习与进度追踪。

目标：通过故事驱动、可重复的对话练习，降低语言学习门槛，并提供可量化结果。

## ✨ 核心功能

- 剧场对话：多轮场景，支持播放、收藏与分享码
- 角色扮演：多轮评测，实时反馈与总结
- 阅读练习：分段阅读、测验、词汇与语法洞察
- 练习闭环：结果、XP 累计与个人进度
- 工程能力：自动迁移、健康探针、基础限流、请求追踪

## 🧭 架构

```
┌─────────────────────────────┐
│        Tauri 客户端          │
│  React + TS + Vite + Tauri   │
└───────────────┬─────────────┘
                │ GraphQL
┌───────────────▼─────────────┐
│          Go 服务端           │
│   GraphQL API + Services     │
└───────┬─────────┬───────────┘
        │         │
  PostgreSQL     Redis / MQ
 (Supabase)    (Cache / Queue)
```

## 🧱 技术栈

- 前端：React、TypeScript、Vite、Tauri 2
- 后端：Go 1.22、GraphQL、PostgreSQL（Supabase）、Redis、RabbitMQ
- 基础设施：Docker / Docker Compose、Cloudflare Workers + R2
- 测试：Go 单元测试、前端单测、Playwright E2E、k6 性能基线

## 📁 仓库结构

- apps/client：Tauri 2 客户端（Windows/macOS/Android）
- apps/server：Go GraphQL 服务端
- infra：本地依赖与部署脚本
- docs：技术与运维文档
- tests/e2e：前端 E2E

## ⚙️ 配置

后端环境变量模板：docs/env.example.md

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

## 🚀 快速开始

### 方案 A：本地开发

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

### 方案 B：桌面端（Tauri 2）

```
# dev
npm run tauri:dev --workspace apps/client

# build
npm run tauri:build --workspace apps/client
```

## 📦 部署

- 部署指南：docs/deploy.md
- 启动与排障：docs/startup.md
- API 概览：docs/api.md

## ✅ 测试

- 后端：cd apps/server && go test ./...
- 前端：npm run test --workspace apps/client
- E2E：npm run test --workspace tests/e2e
- 性能基线：npm run test:perf（需要 k6）

## 📚 文档

- 启动：docs/startup.md
- 部署：docs/deploy.md
- 环境变量：docs/env.example.md
- API：docs/api.md

## 📜 许可证

详见仓库中的 LICENSE。

## 🖼️ 截图

| 登录 | 首页 | 生成 |
| --- | --- | --- |
| ![登录](docs/images/%E7%99%BB%E5%BD%95%E9%A1%B5.png) | ![首页](docs/images/%E5%AF%B9%E8%AF%9D%E9%A1%B5.png) | ![生成](docs/images/%E7%94%9F%E6%88%90%E9%A1%B5.png) |
