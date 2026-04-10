# LinguaQuest 最终测试报告（当前版本）

## 1. 自动化测试结果

- Backend: `go test ./...` 通过
- Frontend lint/typecheck/unit: 通过
- Frontend build: 通过
- Playwright E2E smoke: 通过（登录页渲染）

## 2. 业务链路冒烟（GraphQL）

已验证链路：
- `register -> login -> updateProfile`
- `courses -> generateTheater -> myTheaters`
- `toggleFavorite -> shareTheater`
- `startRoleplay -> submitRoleplayReply -> endRoleplay`

结果：全链路通过，关键字段（收藏、分享码、角色扮演状态）均按预期返回。

## 3. 健康探针

- `GET /healthz` 返回 `ok=true`
- `checks.postgres=up`
- `checks.redis=up`

## 4. 待持续增强项

- E2E 扩展为真实后端联调链路（当前为稳定烟雾用例）
- k6 性能压测纳入 CI 定时任务
- 三端真机回归（Windows/macOS/Android）补充截图与性能记录
