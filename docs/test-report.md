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

## 5. IELTS 生成质量门禁（2026-05-30）

- Backend: `go test ./internal/service -run TestSmallSampleQualityGate -v` 通过
- Backend: `go test ./...` 通过
- Frontend: `npm run typecheck --workspace apps/client` 通过
- Frontend: `npm run lint --workspace apps/client` 通过

门禁覆盖：
- 5 个听力样本：覆盖 Band 5.0、6.0、6.5、7.0 与 Section 1/2/3/4
- 5 个阅读样本：覆盖 Multiple Choice、Matching Information、TFNG、Summary Completion、Mixed Question Set
- 自动检查 prompt 泄漏、英文空格粘连、阅读词数/段落数、题型结构、Band/Section 元数据、fallback 可用性
- 音频链路覆盖 TTS retry 与阅读音频分片续跑单测

结论：代码级质量门禁通过，可以进入真实 AI 小样本生成与人工验收；仍不建议直接扩到 300 篇，需先人工检查 5 篇听力 + 5 篇阅读真实生成结果。

## 6. IELTS fallback 优化复测（2026-05-31）

- Backend: `go test ./...` 通过
- 新增门禁：阅读 fallback 的 `Evidence` 必须能在 passage 中定位；Mixed Question Set 必须保持 `Multiple Choice -> Matching Information -> TFNG -> Summary Completion -> Multiple Choice`
- GraphQL 后台实测样本：`dee3afa6-ffe4-4849-a9d8-befe8dc8fac4`
- 样本配置：`[Band 7.0][Mixed Question Set] public health data governance retest`

复测结果：
- 生成链路：AI 主链两次返回缺少 quiz，服务自动降级到 structured fallback
- fallback 质量：1099 words，9 paragraphs，5 questions
- 题型顺序：`Multiple Choice | Matching Information | TFNG | Summary Completion | Multiple Choice`
- 重复题数：0
- 缺失证据锚点：0

音频状态：
- 文本生成已完成，`audioStatus=PENDING`
- 小米 TTS 返回 `401 Invalid API Key`，属于 provider 鉴权问题；不计入文本质量失败
