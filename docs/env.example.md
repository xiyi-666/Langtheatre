# 环境变量模板

## 后端 (`apps/server`)

- `APP_EDITION=COMMERCIAL`（可选：`OPEN_SOURCE` / `MINI_PROGRAM`；开源版强制移除点数、会员、支付与广告机制，小程序版完全免费、仅保留广告）
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
- `ASR_PROVIDER=XIAOMI`（支持 `XIAOMI` / `ALIYUN` / `DOUBAO` / `GEMINI` / `MINIMAX` / `OPENAI` / `OPENAI_COMPATIBLE`）
- `ASR_API_URL=https://api.xiaomimimo.com/v1`
- `ASR_API_KEY=`
- `ASR_APP_ID=`（仅豆包大模型 ASR 需要）
- `ASR_MODEL=mimo-v2.5-asr`
- `SMTP_HOST=smtp.qq.com`
- `SMTP_PORT=465`（QQ 邮箱 SSL）
- `SMTP_USERNAME=你的QQ邮箱`
- `SMTP_PASSWORD=QQ邮箱授权码`（不是网页登录密码）
- `SMTP_FROM=你的QQ邮箱`
- `PUBLIC_APP_URL=https://你的前端域名`（邮件验证/重置链接使用此地址）
- `EMAIL_VERIFICATION_REQUIRED=true`
- `GENERATION_CONCURRENCY=30`
- `BACKGROUND_TASK_TIMEOUT_SECONDS=1200`
- `HTTP_RATE_LIMIT_PER_MINUTE=180`（所有 HTTP 请求的单 IP 上限）
- `AUTH_RATE_LIMIT_PER_MINUTE=12`（登录、注册、邮箱验证与找回账号的单 IP 上限）
- `AI_REQUEST_RATE_LIMIT_PER_MINUTE=20`（生成、评分、音色与角色对话等 AI 操作的单 IP 上限）
- `GRAPHQL_MAX_BODY_BYTES=16777216`（GraphQL 请求体上限 16 MiB，支持语音练习上传）
- `MEDIA_PROXY_MAX_BYTES=20971520`（媒体代理单响应上限 20 MiB）
- `TRUST_PROXY_HEADERS=false`（仅当反向代理已覆盖外部传入的 `X-Forwarded-For` 时才设为 `true`）
- `ANALYTICS_ENABLED=true`（记录仅后端可见的按日模型、功能和点击聚合）
- `ANALYTICS_TIMEZONE=Asia/Shanghai`（统计按日切分的时区）
- `ANALYTICS_ADMIN_TOKEN=replace-with-a-long-random-secret`（启用内部统计查询接口；不得放入前端环境变量）
- `BILLING_ENABLED=true`（未配置易支付时可设为 `false`，学习功能不受影响）
- `BILLING_FREE_DAILY_CREDITS=20`
- `MINI_AD_FREE_DAILY_USES=3`（仅小程序版：每日前几次 AI 使用不展示广告）
- `MINI_PROGRAM_DAILY_AI_USES=20`（仅小程序公测版：单账号每日 AI 使用总上限）
- `MINI_PROGRAM_AI_COOLDOWN_SECONDS=12`（仅小程序公测版：同账号两次 AI 请求最短间隔）
- `MINI_PROGRAM_MAX_ACTIVE_TASKS=2`（仅小程序公测版：单账号同时排队或执行的 AI 任务数）
- `BILLING_TIMEZONE=Asia/Shanghai`
- `EPAY_GATEWAY_URL=https://你的易支付域名/submit.php`
- `EPAY_MERCHANT_ID=你的商户PID`
- `EPAY_KEY=你的易支付密钥`
- `EPAY_NOTIFY_URL=https://你的后端域名/payments/easypay/notify`（必须可被易支付服务器访问）
- `EPAY_DEFAULT_CHANNEL=alipay`（可选 `alipay` / `wxpay` / `qqpay`）
- `EPAY_SIGNATURE_MODE=RAW_KEY`（标准易支付签名；仅旧网关需要时使用 `KEY_VALUE`）
- `AD_PROVIDER=MOCK`（接入联盟后填写联盟标识；`NONE` 可关闭广告）
- `AD_SCRIPT_URL=`（联盟 Web 广告脚本地址，使用受信任 HTTPS 地址）
- `AD_COURSE_SLOT=`、`AD_LIBRARY_SLOT=`、`AD_RESULT_SLOT=`（三处非打扰广告位 ID）
- `R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com`
- `R2_ACCESS_KEY=xxx`
- `R2_SECRET_KEY=xxx`
- `R2_BUCKET=linguaquest-media`
- `SENTRY_DSN=xxx`

说明：Supabase 直连地址通常要求 IPv6 网络；如果部署环境不支持 IPv6，建议使用 Supabase 提供的连接池地址（pooler）或将应用部署到支持 IPv6 的运行环境。

小米 ASR 使用 `mimo-v2.5-asr` 与 `api-key` 鉴权，接口仅接收 WAV/MP3 Data URL。官方明确支持中文、英文与 `auto`；剧场中的粤语请求会使用 `auto`。阿里云使用 DashScope 的 `qwen3-asr-flash` 兼容接口；豆包大模型 ASR 需填写 `ASR_APP_ID` 与 `ASR_API_KEY`（Access Token）；Gemini 使用 API Key 直接提交短音频；MiniMax 使用其 Audio Transcriptions 兼容接口。

商业版付费商品为：免费版每日 20 点；轻享月卡 ¥9.9 / 800 点、进阶月卡 ¥19.9 / 2,000 点、沉浸月卡 ¥39.9 / 4,800 点；永久会员 ¥199，永久去广告并每 30 天重置 1,200 点。AI 服务耗点为：阅读 20、音色设计 12、剧场 10、写作评分 6、角色对话每轮 2 点。易支付通知按标准 MD5 参数签名验签；同一订单回调可安全重放。

小程序版设为 `APP_EDITION=MINI_PROGRAM`，只需配置 `AD_*` 广告联盟参数；不会暴露任何支付或会员商品，不消耗 AI 点数。默认每天前 3 次 AI 使用不展示广告，之后显示广告位；使用 `MINI_AD_FREE_DAILY_USES` 调整次数。为保护公测 AI 配额，默认还会限制每账号每天 20 次 AI 使用、每 12 秒一次请求、同时最多 2 个后台任务；可通过 `MINI_PROGRAM_*` 参数按实际成本调整。

## 后端运营统计

启用后，服务按天写入模型 Token 账本（供应商、模型、操作、输入/输出/总 Token、请求数、错误数、累计耗时）以及功能使用和点击计数。数据库中只保留日聚合，不记录用户 ID、提示词、文章内容、音频或原始点击流水。

前端只会向 `POST /telemetry/event` 静默发送固定的事件名，必须携带登录令牌；页面不读取或展示统计数据。设置 `ANALYTICS_ADMIN_TOKEN` 后，运维人员可从受保护接口读取聚合报表：

```powershell
curl -H "X-Analytics-Token: <你的统计密钥>" "https://你的后端域名/internal/analytics/daily?from=2026-08-01&to=2026-08-07"
```

未设置 `ANALYTICS_ADMIN_TOKEN` 时，`/internal/analytics/daily` 不会注册。SQLite 与 PostgreSQL 都会持久化统计；内存存储仅适用于本地调试，重启后数据会丢失。

## 前端 (`apps/client`)

- 本地开发：`VITE_API_URL=http://localhost:8177/graphql`
- 桌面端 / Android 发布：`VITE_API_URL=https://api.example.com/graphql`（必须显式配置；发布构建要求 HTTPS）
- 仅本地桌面调试可设置 `TAURI_LOCAL_API_DEV=1` 并使用 `http://localhost:<port>/graphql` 或 `http://127.0.0.1:<port>/graphql`
- Docker Web 部署：`VITE_API_URL=/graphql`
- `VITE_SENTRY_DSN=xxx`
- `VITE_APP_EDITION=COMMERCIAL`（开源构建使用 `OPEN_SOURCE`；小程序构建使用 `MINI_PROGRAM`）
