# LinguaQuest

LinguaQuest 是一个使用 Tauri 2 + React + TypeScript + Go 构建的跨平台语言学习应用。

## 目录结构

- `apps/client`: Tauri 2 客户端（Windows/macOS/Android）
- `apps/server`: Go GraphQL 服务
- `infra`: 本地依赖与部署脚本
- `docs`: 技术与运维文档
- `tests/e2e`: 前端关键路径 E2E

## 仓库上传约定

以下目录/文件为本地开发或流程产物，默认不上传到 GitHub：

- `node_modules/`
- `test/`、`tests/` 与测试文件（如 `*.test.ts`、`*.spec.ts`、`*_test.go`）
- `.cursor/`
- `.spec-workflow/`
- `demand/`

如需临时上传，请先评估是否包含隐私信息或本地中间产物。

## 快速开始

1. 安装 Node.js 20+、Go 1.22+、Rust stable。
2. 安装依赖：`npm install`（根目录）与 `go mod tidy`（`apps/server`）。
3. 设置后端环境变量：`SUPABASE_DB_URL`（Supabase PostgreSQL 连接串）与 `JWT_SECRET`。
4. 启动基础依赖（Redis/RabbitMQ）：`docker compose -f infra/docker-compose.yml up -d`
5. 启动后端：`go run ./cmd/server`（在 `apps/server`，启动时会自动执行 `migrations` 目录下 SQL）
6. 启动前端：`npm run client:dev`

## 健康探针

- 接口：`GET /healthz`
- 返回：PostgreSQL 与 Redis 可达性检查（JSON）
- 详细启动与排障说明：`docs/startup.md`

## 产品化模块（当前实现）

- 认证与资料：注册/登录/刷新/登出、资料编辑
- 课程与剧场：课程列表、剧场生成、剧场库、收藏与分享码
- 练习闭环：播放、答题、结算、XP累计
- 角色扮演：开启会话、多轮回复、评分与总结反馈
- 工程能力：自动迁移、请求ID、基础限流、CORS、CI/CD 与健康探针

## 三端构建命令（Tauri 2）

- Windows/macOS 桌面开发：`npm run tauri:dev --workspace apps/client`
- Windows/macOS 桌面构建：`npm run tauri:build --workspace apps/client`
- Android 开发：`npm run tauri:android:dev --workspace apps/client`
- Android 构建：`npm run tauri:android:build --workspace apps/client`

## 测试矩阵

- 后端单测：`cd apps/server && go test ./...`
- 前端单测：`npm run test --workspace apps/client`
- 前端 E2E：`npm run test --workspace tests/e2e`
- 性能基线：`npm run test:perf`（需安装 k6）

## 全端 CI/CD（Web + 桌面 + Android + Docker）

- 工作流：`.github/workflows/release-all.yml`
- 触发方式：
	- 推送 Tag（如 `v0.1.0`）自动触发
	- 在 GitHub Actions 页面手动触发 `release-all`

### 需要配置的 GitHub Secrets

- `DOCKERHUB_USERNAME`：Docker Hub 用户名
- `DOCKERHUB_TOKEN`：Docker Hub Access Token
- `VITE_API_URL_PROD`：前端生产 API 地址（示例：`http://61.244.24.7:8177/graphql` 或 `https://api.yourdomain.com/graphql`）
- `VITE_SENTRY_DSN`：可选，前端 Sentry DSN
- `JWT_SECRET`：生产环境 JWT 密钥
- `SUPABASE_DB_URL`：生产 PostgreSQL 连接串
- `OPENAI_API_KEY`：可选，生产模型 API Key
- `OPENAI_MODEL`：可选，默认 `gpt-4o-mini`
- `OPENAI_BASE_URL`：可选，默认 `https://api.openai.com`
- `TTS_API_URL`：可选，生产 TTS 服务地址
- `TTS_API_KEY`：可选，生产 TTS 服务密钥
- `TTS_VOICE`：可选，默认 `female-1`
- `TTS_USE_UPLOAD_PROMPT`：可选，默认 `false`
- `TTS_PROMPT_AUDIO_PATH`：可选，TTS prompt 音频路径
- `TTS_RETURN_JSON`：可选，默认 `true`
- `TTS_TIMEOUT_SECONDS`：可选，生产 TTS 超时时间（秒，默认 `45`）
- `TTS_MAX_RETRIES`：可选，生产 TTS 重试次数（默认 `1`）

### 发布产物

- `web-dist`：Web 静态站点构建产物
- `tauri-windows-latest`：Windows 桌面安装包
- `tauri-macos-latest`：macOS 桌面安装包
- `android-apk`：Android APK 产物
- Docker 镜像：`<DOCKERHUB_USERNAME>/linguaquest-server` 与 `<DOCKERHUB_USERNAME>/linguaquest-client`
