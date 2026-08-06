# LinguaQuest 版本发行

项目使用同一学习核心维护三种产品配置，避免模型、TTS、ASR 和学习功能在多个发行版本中漂移。

| 能力 | 开源版 | 商业版 | 小程序版 |
| --- | --- | --- | --- |
| 模型、TTS、ASR 配置 | 支持 | 支持 | 支持 |
| AI 点数与扣点 | 不启用 | 支持 | 不启用 |
| 广告 | 不暴露 | 支持配置 | 支持配置；每日前几次 AI 使用免广告 |

所有发行版都会保留 HTTP 请求体限制、真实 IP 全局限流、认证操作专用限流和媒体代理 SSRF 防护。小程序公测版额外启用账号级 AI 公平使用限制。
| 会员订阅 | 不提供 | 支持 | 不提供 |
| 易支付 | 不注册 | 订阅与买断 | 不提供 |
| 易支付回调路由 | 不注册 | `/payments/easypay/notify` | 不注册 |

## 开源版

后端使用 `APP_EDITION=OPEN_SOURCE`。该模式会忽略所有 `BILLING_*`、`EPAY_*` 与广告配置，并从 GraphQL schema 移除相关查询和 mutation。

前端使用：

```powershell
npm --prefix apps/client run build:open-source
```

构建模式为 `open-source`，会员入口、广告请求和耗点提示均不会渲染。

## 商业版

后端使用 `APP_EDITION=COMMERCIAL`，并在生产环境配置 `BILLING_ENABLED=true`、`EPAY_*`、广告联盟脚本和广告位 ID。

前端使用：

```powershell
npm --prefix apps/client run build:commercial
```

商业版会显示每次 AI 服务的实际耗点；任务排队、保存或生成失败会退款。

## 小程序版

后端使用 `APP_EDITION=MINI_PROGRAM`，仅保留广告联盟配置。该版本不返回会员商品、不会暴露任何支付 mutation，也不消耗 AI 点数。

前端使用：

```powershell
npm --prefix apps/client run build:mini-program
```

小程序版完全免费。默认每天前 3 次 AI 使用不返回广告位，后续使用显示已配置的广告联盟广告；可通过 `MINI_AD_FREE_DAILY_USES` 调整该阈值。任务排队失败或生成失败会回退一次使用计数。

公测防滥用默认值为：每账号每天 20 次 AI 使用、每 12 秒最多发起一次 AI 请求、同一账号同时最多 2 个后台任务；并叠加单 IP 每分钟 20 次 AI 操作限制。对应环境变量为 `MINI_PROGRAM_DAILY_AI_USES`、`MINI_PROGRAM_AI_COOLDOWN_SECONDS`、`MINI_PROGRAM_MAX_ACTIVE_TASKS`、`AI_REQUEST_RATE_LIMIT_PER_MINUTE`。

> `APP_EDITION` 是运行时功能隔离。若需要把商业实现本身保持为闭源，请只向公开仓库发布开源发行分支或导出的开源包，并将当前商业源码保留在私有仓库。
