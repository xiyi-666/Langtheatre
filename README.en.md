# LinguaQuest

English | [简体中文](README.md)

A cross-platform language learning app that combines theater-style dialogues, reading practice, and roleplay to deliver a full practice loop (listen → answer → results → XP).

Quick Start • Overview • Architecture • Configuration • Getting Started • Deployment & Docs

## 📝 Overview

LinguaQuest is built with Tauri 2 + React + TypeScript + Go. It targets English and Cantonese practice scenarios, supporting theater generation, roleplay, reading exercises, and progress tracking.

Goal: lower the barrier to language learning through story-driven, replayable dialogues with measurable outcomes.

## ✨ Key Features

- Theater dialogues: multi-turn scenes with playback, favorites, and share codes
- Roleplay: multi-round evaluation with real-time feedback and summary
- Reading practice: segmented reading, quizzes, vocabulary and grammar insights
- Practice loop: results, XP accumulation, personal progress
- Engineering: auto migrations, health probes, basic rate limiting, request tracing

## 🧭 Architecture

```
┌─────────────────────────────┐
│        Tauri Client          │
│  React + TS + Vite + Tauri   │
└───────────────┬─────────────┘
                │ GraphQL
┌───────────────▼─────────────┐
│          Go Server           │
│   GraphQL API + Services     │
└───────┬─────────┬───────────┘
        │         │
  PostgreSQL     Redis / MQ
 (Supabase)    (Cache / Queue)
```

## 🧱 Tech Stack

- Frontend: React, TypeScript, Vite, Tauri 2
- Backend: Go 1.22, GraphQL, PostgreSQL (Supabase), Redis, RabbitMQ
- Infra: Docker / Docker Compose, Cloudflare Workers + R2
- Testing: Go unit tests, frontend unit tests, Playwright E2E, k6 perf baseline

## 📁 Repository Structure

- apps/client: Tauri 2 client (Windows/macOS/Android)
- apps/server: Go GraphQL server
- infra: local dependencies and deployment scripts
- docs: technical and ops docs
- tests/e2e: frontend E2E

## ⚙️ Configuration

Backend env template: docs/env.example.md

Common settings (apps/server/.env):

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

Frontend env example:

```
VITE_API_URL=http://localhost:8080/graphql
```

## 🚀 Quick Start

### Option A: Local Development

1) Install dependencies

```
# project root
npm install

# backend deps
cd apps/server
go mod tidy
```

2) Start infra dependencies (Redis / RabbitMQ)

```
docker compose -f infra/docker-compose.yml up -d redis rabbitmq
```

3) Start backend

```
cd apps/server
go run ./cmd/server
```

4) Start frontend

```
# project root
npm run client:dev
```

5) Health checks

```
GET http://localhost:8080/healthz
GET http://localhost:8080/readyz
```

### Option B: Desktop (Tauri 2)

```
# dev
npm run tauri:dev --workspace apps/client

# build
npm run tauri:build --workspace apps/client
```

## 📦 Deployment

- Deployment guide: docs/deploy.md
- Startup & troubleshooting: docs/startup.md
- API overview: docs/api.md

## ✅ Tests

- Backend: cd apps/server && go test ./...
- Frontend: npm run test --workspace apps/client
- E2E: npm run test --workspace tests/e2e
- Perf baseline: npm run test:perf (requires k6)

## 📚 Docs

- Startup: docs/startup.md
- Deployment: docs/deploy.md
- Environment variables: docs/env.example.md
- API: docs/api.md

## 📜 License

See LICENSE in the repository.

## 🖼️ Screenshots

| Login | Home | Generate |
| --- | --- | --- |
| ![Login](docs/images/%E7%99%BB%E5%BD%95%E9%A1%B5.png) | ![Home](docs/images/%E5%AF%B9%E8%AF%9D%E9%A1%B5.png) | ![Generate](docs/images/%E7%94%9F%E6%88%90%E9%A1%B5.png) |
