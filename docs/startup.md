# LinguaQuest 启动文档（Supabase + Redis）

## 1. 前置条件

- 安装 Node.js 20+、Go 1.22+、Docker。
- 准备 Supabase PostgreSQL 连接串（建议使用环境变量注入）。

## 2. 环境变量配置

在 `apps/server/.env` 中配置：

```
PORT=8080
JWT_SECRET=dev-secret-change-me
REDIS_ADDR=localhost:6379
SUPABASE_DB_URL=postgresql://postgres:[YOUR-PASSWORD]@db.jctsgtqtwuwbxyrxgwgy.supabase.co:5432/postgres
MIGRATIONS_DIR=migrations
OPENAI_API_KEY=
OPENAI_MODEL=gpt-4o-mini
OPENAI_BASE_URL=https://api.openai.com
TTS_API_URL=
TTS_API_KEY=
TTS_VOICE=female-1
```

说明：
- 兼容变量名：`SUPABASE_DB_URL`、`SUPBASE_DB_URL`、`DATABASE_URL`。
- 若未配置数据库变量，服务会回退到内存存储，仅用于本地开发。
- 启动时会自动执行 `MIGRATIONS_DIR` 下的 `.sql` 文件（默认 `migrations`）。
- 数据库密码中若含 `@ : / ? # [ ]` 等字符，必须先 URL 编码再放入连接串（否则会报 `invalid userinfo`）。
- 若配置了 `TTS_API_URL`，后端会在剧场生成时调用外部 TTS 接口并写入 `dialogues.audioUrl`。
- 可通过 `OPENAI_BASE_URL` 使用自建网关或代理地址（默认官方 API）。

## 3. 启动 Redis / RabbitMQ

在项目根目录执行：

```
docker compose -f infra/docker-compose.yml up -d redis rabbitmq
```

## 4. 启动后端

```
cd apps/server
go mod tidy
go run ./cmd/server
```

## 5. 探针验证（可达性检查）

启动后访问：

```
GET http://localhost:8080/healthz
GET http://localhost:8080/readyz
```

返回示例：

```json
{
  "ok": true,
  "timestamp": "2026-03-25T09:00:00Z",
  "checks": {
    "postgres": "up",
    "redis": "up"
  }
}
```

当任一依赖不可达时，HTTP 状态码为 `503`，并在 `checks` 中显示 `down: <error>`。

## 6. 启动前端（可选）

在项目根目录执行：

```
npm install
npm run client:dev
```
