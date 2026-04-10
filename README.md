# LinguaQuest

LinguaQuest 是一个使用 Tauri 2 + React + TypeScript + Go 构建的跨平台语言学习应用。

## 目录结构

- `apps/client`: Tauri 2 客户端（Windows/macOS/Android）
- `apps/server`: Go GraphQL 服务
- `infra`: 本地依赖与部署脚本
- `docs`: 技术与运维文档
- `tests/e2e`: 前端关键路径 E2E

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
