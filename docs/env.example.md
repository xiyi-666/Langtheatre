# 环境变量模板

## 后端 (`apps/server`)

- `PORT=8177`
- `JWT_SECRET=replace-with-production-secret`
- `REDIS_ADDR=localhost:6379`
- `SUPABASE_DB_URL=postgresql://postgres:YOUR_PASSWORD_URLENCODED@db.jctsgtqtwuwbxyrxgwgy.supabase.co:5432/postgres`
- `DATABASE_URL=postgresql://postgres:YOUR_PASSWORD_URLENCODED@db.jctsgtqtwuwbxyrxgwgy.supabase.co:5432/postgres`（可选）
- `RABBITMQ_URL=amqp://guest:guest@localhost:5672/`
- `OPENAI_API_KEY=`
- `OPENAI_MODEL=gpt-5.4`
- `OPENAI_BASE_URL=http://43.172.5.210:3000/v1`（可替换为你自己的网关）
- `TTS_PROVIDER=XIAOMI`（可选：`CUSTOM` / `XIAOMI`）
- `TTS_API_URL=https://api.xiaomimimo.com/v1`
- `TTS_API_KEY=`
- `TTS_VOICE=mimo_default`（也可切换为 `冰糖` / `Chloe` 等官方音色）
- `TTS_MODEL=mimo-v2.5-tts`
- `TTS_AUDIO_FORMAT=mp3`
- `TTS_MAX_CONCURRENCY=2`
- `MEDIA_DIR=media`
- `TTS_TIMEOUT_SECONDS=300`
- `TTS_MAX_RETRIES=1`
- `R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com`
- `R2_ACCESS_KEY=xxx`
- `R2_SECRET_KEY=xxx`
- `R2_BUCKET=linguaquest-media`
- `SENTRY_DSN=xxx`

说明：Supabase 直连地址通常要求 IPv6 网络；如果部署环境不支持 IPv6，建议使用 Supabase 提供的连接池地址（pooler）或将应用部署到支持 IPv6 的运行环境。

## 前端 (`apps/client`)

- 本地开发：`VITE_API_URL=http://localhost:8177/graphql`
- 桌面端 / Android 发布：`VITE_API_URL=http://61.244.24.7/graphql`
- Docker Web 部署：`VITE_API_URL=/graphql`
- `VITE_SENTRY_DSN=xxx`
