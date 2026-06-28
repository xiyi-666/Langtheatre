# LinguaQuest Docker 部署手册

## 1. 部署目标

- 服务器地址：`61.244.24.7`
- 对外统一入口：`http://61.244.24.7`
- Web、Windows、Linux、macOS、Android 客户端统一访问：

```text
http://61.244.24.7/graphql
```

- Docker 部署模式：
  - `client` 容器对外暴露 `80`
  - `server` 容器仅在 Docker 网络内监听 `8177`
  - `/graphql`、`/healthz`、`/readyz`、`/media-proxy` 全部由 `client` 容器反向代理到 `server`

## 2. GitHub Actions 约定

### 自动部署流程

- 工作流：`.github/workflows/deploy.yml`
- 触发条件：
  - 推送到 `main`
  - 手动触发 `workflow_dispatch`

执行内容：

1. 构建并推送 `linguaquest-server` Docker 镜像到 Docker Hub
2. 构建并推送 `linguaquest-client` Docker 镜像到 Docker Hub
3. 生成部署 bundle：
   - `docker-compose.deploy.yml`
   - `.env.production`
   - 渲染后的 compose 配置
4. 若配置了 SSH 秘钥，则自动 SSH 到 `61.244.24.7` 执行：
   - `docker compose pull`
   - `docker compose up -d --remove-orphans`
5. 若未配置 `DEPLOY_USER` / `DEPLOY_SSH_KEY`，workflow 仍会构建并推送镜像、生成部署 bundle，但不会执行远程重启

### 发布打包流程

- 工作流：`.github/workflows/release-all.yml`
- 触发条件：
  - 推送 tag：`v*`
  - 手动触发 `workflow_dispatch`

执行内容：

1. 自动构建 Tauri 桌面端：
   - Windows
   - Linux
   - macOS
2. 自动构建 Android APK
3. 自动推送 Docker 镜像
4. 自动创建 GitHub Release 并上传构建产物

## 3. 必要 Secrets / Variables

### GitHub Secrets

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`
- `JWT_SECRET`
- `SUPABASE_DB_URL`
- `OPENAI_API_KEY`
- `TTS_API_KEY`
- `SENTRY_DSN`
- `VITE_SENTRY_DSN`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`

Android release 签名可选：

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

未配置上述 Android 签名 secrets 时，release workflow 会退回为调试版 APK 构建。

### GitHub Variables

- `OPENAI_MODEL`
- `OPENAI_BASE_URL`
- `TTS_PROVIDER`
- `TTS_API_URL`
- `TTS_VOICE`
- `TTS_MODEL`
- `TTS_AUDIO_FORMAT`
- `TTS_MAX_CONCURRENCY`
- `MEDIA_DIR`
- `TTS_USE_UPLOAD_PROMPT`
- `TTS_PROMPT_AUDIO_PATH`
- `TTS_RETURN_JSON`
- `TTS_TIMEOUT_SECONDS`
- `TTS_MAX_RETRIES`

## 4. 服务器前置条件

在 `61.244.24.7` 上准备：

1. Docker
2. Docker Compose Plugin
3. 可执行 `docker login`
4. 一个部署目录，例如：

```bash
sudo mkdir -p /opt/linguaquest
sudo chown -R $USER:$USER /opt/linguaquest
```

## 5. 手动部署兜底命令

即使不走自动 SSH 部署，也可以在服务器上手动执行：

```bash
cd /opt/linguaquest
docker compose --env-file .env.production -f docker-compose.deploy.yml pull
docker compose --env-file .env.production -f docker-compose.deploy.yml up -d --remove-orphans
```

## 6. 部署后检查

检查入口：

```bash
curl http://61.244.24.7/healthz
curl http://61.244.24.7/readyz
```

检查 GraphQL：

```bash
curl http://61.244.24.7/graphql
```

检查桌面/移动端：

- Windows / Linux / macOS / Android 构建产物默认请求 `http://61.244.24.7/graphql`
- 若后续切换域名或 HTTPS，只需在 release workflow 或 `VITE_API_URL` 中统一修改
