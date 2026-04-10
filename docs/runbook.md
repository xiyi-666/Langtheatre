# LinguaQuest 运行与回滚手册

## 启动检查

- `/healthz` 返回 `200` 且 `checks.postgres=up`、`checks.redis=up`
- `/readyz` 返回 `200`，用于部署后就绪探测
- GraphQL `register/login` 可用
- Supabase PostgreSQL/Redis/RabbitMQ 连接成功

## 故障排查

1. **登录失败**：检查 JWT_SECRET、时钟偏差与 Redis 状态。
2. **健康探针异常**：检查 `/healthz` 返回 JSON 中的 `checks` 字段，定位 postgres 或 redis。
3. **生成超时**：检查 RabbitMQ 队列堆积与第三方 AI/TTS 限流。
4. **音频播放失败**：检查 R2 对象权限、CDN 缓存与 URL 签名有效期。

## 回滚流程

1. 在发布系统中切换到上一个稳定版本
2. 保持数据库 schema 向后兼容（仅允许前向可兼容迁移）
3. 回滚后执行冒烟：登录 -> 生成 -> 播放 -> 提交答案

## 值班响应

- P0（全站不可用）10分钟内响应，30分钟内止血
- P1（关键路径降级）30分钟内响应，2小时内恢复
