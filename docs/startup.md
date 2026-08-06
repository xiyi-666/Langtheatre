# LinguaQuest 启动文档（Supabase + Redis）

## 1. 前置条件

- 安装 Node.js 20+、Go 1.22+、Docker。
- 准备 Supabase PostgreSQL 连接串（建议使用环境变量注入）。

## 2. 环境变量配置

在 `apps/server/.env` 中配置：

```
PORT=8177
APP_EDITION=COMMERCIAL
JWT_SECRET=dev-secret-change-me
REDIS_ADDR=localhost:6379
SUPABASE_DB_URL=postgresql://postgres:[YOUR-PASSWORD]@db.jctsgtqtwuwbxyrxgwgy.supabase.co:5432/postgres
MIGRATIONS_DIR=migrations
OPENAI_API_KEY=
OPENAI_MODEL=gpt-5.4
OPENAI_BASE_URL=http://43.172.5.210:3000/v1
TTS_PROVIDER=XIAOMI
TTS_API_URL=https://api.xiaomimimo.com/v1
TTS_API_KEY=
TTS_VOICE=mimo_default
TTS_MODEL=mimo-v2.5-tts
TTS_AUDIO_FORMAT=mp3
TTS_MAX_CONCURRENCY=2
MEDIA_DIR=media
TTS_TIMEOUT_SECONDS=300
TTS_MAX_RETRIES=1
ASR_PROVIDER=XIAOMI
ASR_API_URL=https://api.xiaomimimo.com/v1
ASR_API_KEY=
ASR_APP_ID=
ASR_MODEL=mimo-v2.5-asr
SMTP_HOST=smtp.qq.com
SMTP_PORT=465
SMTP_USERNAME=your-qq-number@qq.com
SMTP_PASSWORD=your-qq-mail-authorization-code
SMTP_FROM=your-qq-number@qq.com
PUBLIC_APP_URL=http://localhost:5174
EMAIL_VERIFICATION_REQUIRED=true
GENERATION_CONCURRENCY=30
BACKGROUND_TASK_TIMEOUT_SECONDS=1200
HTTP_RATE_LIMIT_PER_MINUTE=180
AUTH_RATE_LIMIT_PER_MINUTE=12
AI_REQUEST_RATE_LIMIT_PER_MINUTE=20
GRAPHQL_MAX_BODY_BYTES=16777216
MEDIA_PROXY_MAX_BYTES=20971520
TRUST_PROXY_HEADERS=false
ANALYTICS_ENABLED=true
ANALYTICS_TIMEZONE=Asia/Shanghai
ANALYTICS_ADMIN_TOKEN=replace-with-a-long-random-secret
```

说明：
- 兼容变量名：`SUPABASE_DB_URL`、`SUPBASE_DB_URL`、`DATABASE_URL`。
- 若未配置数据库变量，服务会回退到内存存储，仅用于本地开发。
- 启动时会自动执行 `MIGRATIONS_DIR` 下的 `.sql` 文件（默认 `migrations`）。
- 数据库密码中若含 `@ : / ? # [ ]` 等字符，必须先 URL 编码再放入连接串（否则会报 `invalid userinfo`）。
- 若配置了 `TTS_API_URL`，后端会在剧场生成时调用外部 TTS 接口并写入 `dialogues.audioUrl`。
- 可通过 `OPENAI_BASE_URL` 使用自建网关或代理地址（默认官方 API）。
- QQ 邮箱需先在邮箱设置中开启 SMTP 服务，并将生成的**授权码**填入 `SMTP_PASSWORD`；请勿使用网页登录密码。
- `PUBLIC_APP_URL` 必须是用户可访问的前端地址，邮件中的验证与重置链接会跳转至该地址。
- 生产环境请保持 `EMAIL_VERIFICATION_REQUIRED=true`；本地未配置 SMTP 时，可临时设为 `false` 进行功能开发。
- 阅读材料和剧场生成均进入后台队列，默认最多同时执行 `GENERATION_CONCURRENCY=30` 个任务；单任务默认 20 分钟超时。
- 角色音色库复用当前小米 TTS 的 `Base URL` 与 `TTS_API_KEY`：保持小米 TTS 已配置后，可在个人中心通过 VoiceDesign 提示词创建可试听的角色音色；剧场生成时可选择自动匹配或从音色库分配。
- 剧场语音练习会在后台按“ASR → AI 续聊 → TTS”处理。小米、阿里云 DashScope、豆包、Gemini、MiniMax、OpenAI 和 OpenAI 兼容接口均可在个人中心选择。小米 ASR 仅接收 WAV/MP3；前端会将浏览器录音转换为 WAV，并限制为 10MB / 90 秒。豆包额外需要 App ID 与 Access Token。
- 写作题目生成与评分复用“模型管理”中的 LLM 配置；提交后的评分同样在后台队列处理。
- 对外公测建议使用 `APP_EDITION=MINI_PROGRAM`。默认安全策略为：单 IP 每分钟 180 个总请求、12 次认证操作、20 次 AI 操作；单账号每天最多 20 次 AI 使用、两次 AI 请求间隔 12 秒、同时最多 2 个后台任务。若服务放在 Nginx、Caddy 或 CDN 后，只有确认代理会清洗并重写 `X-Forwarded-For` 后才设置 `TRUST_PROXY_HEADERS=true`。
- 后端会将模型 Token、成功发起的功能使用和导航点击按天聚合保存，不保存用户内容或原始点击流水。统计不会出现在 GraphQL 或产品页面；配置 `ANALYTICS_ADMIN_TOKEN` 后，使用 `GET /internal/analytics/daily?from=YYYY-MM-DD&to=YYYY-MM-DD` 并携带 `X-Analytics-Token` 请求头读取运维报表。

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

开源版将 `APP_EDITION=OPEN_SOURCE` 写入环境后再启动；它不会注册会员、广告、支付或点数接口。商业版使用 `APP_EDITION=COMMERCIAL`；小程序版使用 `APP_EDITION=MINI_PROGRAM`，完全免费且仅通过广告支持运营，默认每天前 3 次 AI 使用不展示广告，并启用公测公平使用限制。前端分别使用 `npm --prefix apps/client run build:open-source`、`npm --prefix apps/client run build:commercial` 与 `npm --prefix apps/client run build:mini-program` 构建三个发行版本，详情见 `docs/editions.md`。

## 5. 探针验证（可达性检查）

启动后访问：

```
GET http://localhost:8177/healthz
GET http://localhost:8177/readyz
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

本地默认访问地址为 `http://localhost:5174`。完成注册后，LinguaQuest 邮件会使用相同的品牌图标发送验证、重置密码与找回用户名通知。
