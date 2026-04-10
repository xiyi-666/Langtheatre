# 环境变量模板

## 后端 (`apps/server`)

- `PORT=8080`
- `JWT_SECRET=replace-with-production-secret`
- `REDIS_ADDR=localhost:6379`
- `SUPABASE_DB_URL=postgresql://postgres:YOUR_PASSWORD_URLENCODED@db.jctsgtqtwuwbxyrxgwgy.supabase.co:5432/postgres`
- `DATABASE_URL=postgresql://postgres:YOUR_PASSWORD_URLENCODED@db.jctsgtqtwuwbxyrxgwgy.supabase.co:5432/postgres`（可选）
- `RABBITMQ_URL=amqp://guest:guest@localhost:5672/`
- `OPENAI_API_KEY=`
- `OPENAI_MODEL=gpt-4o-mini`
- `OPENAI_BASE_URL=https://api.openai.com`（可替换为你自己的网关）
- `TTS_API_URL=`
- `TTS_API_KEY=`
- `TTS_VOICE=female-1`
- `R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com`
- `R2_ACCESS_KEY=xxx`
- `R2_SECRET_KEY=xxx`
- `R2_BUCKET=linguaquest-media`
- `SENTRY_DSN=xxx`

说明：Supabase 直连地址通常要求 IPv6 网络；如果部署环境不支持 IPv6，建议使用 Supabase 提供的连接池地址（pooler）或将应用部署到支持 IPv6 的运行环境。

## 前端 (`apps/client`)

- `VITE_API_URL=http://localhost:8080/graphql`
- `VITE_SENTRY_DSN=xxx`
