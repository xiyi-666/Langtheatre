# LinguaQuest Docker 部署手册

## 1. 部署目标

- 服务器地址：`61.244.24.7`
- 对外统一入口：`https://langquest.cloudaihub.dpdns.org`
- Web、Windows、Linux、macOS、Android 客户端统一访问：

```text
https://langquest.cloudaihub.dpdns.org/graphql
```

- Docker 部署模式：
  - `client` 容器对外暴露 `80`
  - `server` 容器仅在 Docker 网络内监听 `8177`
  - `/graphql`、`/healthz`、`/readyz`、`/media-proxy` 全部由 `client` 容器反向代理到 `server`

## 2. GitHub Actions 约定

### 镜像发布流程

- 工作流：`.github/workflows/deploy.yml`
- 触发条件：
  - 推送到 `main`
  - 手动触发 `workflow_dispatch`

执行内容：

1. 校验前端和后端代码
2. 构建并推送 `linguaquest-server:mini-program` 到 Docker Hub
3. 构建并推送 `linguaquest-client:mini-program` 到 Docker Hub
4. 同时推送 `mini-<commit sha>` 标签，供需要时固定或回滚版本

该工作流不会连接线上服务器，不会上传 `.env.production`，也不会执行 `docker compose up`。服务器更新完全由管理员手动控制。

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

## 3. GitHub Actions 配置

### GitHub Secrets

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`
- `VITE_SENTRY_DSN`

线上运行时使用的 JWT、数据库、模型、TTS、ASR、SMTP、统计和支付密钥不需要配置到镜像发布工作流。这些值只保存在服务器的 `/opt/linguaquest/.env.production`。

Android release 签名可选：

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

未配置上述 Android 签名 secrets 时，release workflow 会退回为调试版 APK 构建。

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

首次部署时，将以下文件放入 `/opt/linguaquest`：

- `infra/docker-compose.deploy.yml`，线上文件名保持 `docker-compose.deploy.yml`
- 根据 `infra/.env.mini-program.example` 创建的 `.env.production`
- `infra/deploy-manual.sh`

`.env.production` 至少应使用：

```dotenv
DOCKERHUB_USERNAME=jasper177
CLIENT_IMAGE_TAG=mini-program
SERVER_IMAGE_TAG=mini-program
APP_EDITION=MINI_PROGRAM
REDIS_ADDR=linguaquest-redis:6379
PUBLIC_APP_URL=https://langquest.cloudaihub.dpdns.org
```

## 5. 手动部署

Actions 镜像发布成功后，SSH 登录服务器并执行：

```bash
cd /opt/linguaquest
bash deploy-manual.sh
```

脚本会创建或复用 `linguaquest-network`，连接现有 `linguaquest-redis`，可选连接 `linguaquest-rabbitmq`，然后拉取镜像并更新前后端。脚本不会删除现有 Redis、RabbitMQ、数据卷或数据库。

如 Docker Hub 镜像为私有仓库，首次执行前登录：

```bash
docker login -u jasper177
```

## 6. 部署后检查

检查入口：

```bash
curl https://langquest.cloudaihub.dpdns.org/healthz
curl https://langquest.cloudaihub.dpdns.org/readyz
```

检查 GraphQL：

```bash
curl https://langquest.cloudaihub.dpdns.org/graphql
```

检查桌面/移动端：

- Windows / Linux / macOS / Android 构建必须显式设置 HTTPS 的 `VITE_API_URL`；缺失配置会在打包前失败
- 发布工作流通过 `DESKTOP_API_URL` secret 注入受控 API 地址
