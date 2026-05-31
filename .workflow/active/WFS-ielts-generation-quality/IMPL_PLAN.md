# Implementation Plan: IELTS Generation Quality

## 1. Requirements Summary

本计划用于先修复 IELTS 听力与阅读生成质量，再决定是否批量扩容。当前不执行 300 篇扩容；成功标准是先能稳定生成 5 篇听力 + 5 篇阅读小样本，并通过质量验收。

核心 P0 问题包括：听力英文空格丢失、听力 Band/Section 控制弱、阅读 prompt 泄漏、阅读题型与标签不匹配、阅读 Band 递进弱、fallback 模板质量低、批量生成前缺少质量验收。P1 问题包括：阅读篇幅结构真实度、题目证据定位、TTS timeout 续跑、课程元数据显式化。

## 2. Architecture Decisions

1. 生成质量先落在后端。`apps/server/internal/ai/generator.go` 是模型 prompt、解析和基础校验入口；`apps/server/internal/service/service.go` 是业务编排、fallback、TTS 和阅读材料保存入口。
2. 阅读题型不要继续复用纯 multiple-choice 语义表达所有题型。建议新增阅读专用结构，保留 legacy `question/options/answerKey` 字段，向后兼容旧数据和现有 UI。
3. 元数据采用 additive change：domain 新字段、Postgres migration、新 SQLite 列/兼容迁移、GraphQL 新字段、客户端类型和 query 新字段。旧内容字段为空时通过 topic/level 推断。
4. fallback 不再作为低质量兜底，而是作为可验收的 deterministic generator：按 Band、Section、question_type 生成结构化内容，避免大量雷同。
5. TTS 继续保留现有 provider 和小米设计音色优先，但服务层要支持失败 chunk 的重试/续跑，避免一个 timeout 让整篇材料永久不可用。
6. 批量扩容前必须有质量门禁：自动检查 prompt 泄漏、空格压缩、段落/词数、题型结构、Band 差异和音频状态，再人工验收小样本。

## 3. Task Breakdown

### IMPL-1: Output Hygiene And Quality Guards

建立生成结果的基础卫生层，覆盖英文空格丢失、prompt 泄漏、过短阅读、通用题重复等硬性失败。重点在 `apps/server/internal/ai/generator.go` 和 `apps/server/internal/service/service.go` 增加小而集中的 helper，并补 Go 单测。

Acceptance:
- 能检测 `Goodafternoon,BrookdaleLanguageCentre` 这类英文 token 粘连风险。
- 能过滤/拒绝阅读正文中的 `Task design`、`Create an IELTS Academic reading drill`、`no copied official test content` 等 prompt 泄漏。
- 生成内容进入 TTS/保存前经过统一校验，失败时返回明确错误或走高质量 fallback。

### IMPL-2: Listening Band And Section Profiles

把听力 difficulty 从“题目数开关”升级为 Band profile，并把 Section 1/2/3/4 的任务结构显式化。实现应围绕 `requiredQuizCount`、`OpenAIGenerator.Generate`、`prepareTheaterTopic`、`fallbackGeneratedContent` 分层修改。

Acceptance:
- Band 5.0、6.0、7.0 在句长、词汇、paraphrase、干扰项密度上有可测试差异。
- Section 1 偏表格/数字/拼写，Section 2 偏地图/说明，Section 3 偏观点匹配，Section 4 偏笔记摘要。
- 旧的 topic 字符串仍可工作，但 `[Band]`、`[Section]`、Focus、Task design 能被解析成内部 profile。

### IMPL-3: Reading Metadata And Question Model

为阅读材料增加显式 metadata 和阅读专用 question model，解决“分类藏在 topic 字符串里”和“题型标签与实际题目结构不匹配”的结构性问题。涉及 domain、store、migration、GraphQL、client type/query。

Acceptance:
- ReadingMaterial 显式暴露 `band` / `stage` / `section` / `skillFocus` / `questionType` / `scenarioFamily`。
- 阅读题目支持 Matching Headings、Matching Information、TFNG、Summary Completion、Multiple Choice、Mixed Question Set 等结构，同时兼容旧 `question/options/answerKey` 数据。
- 旧数据库行可读取，未填字段有安全默认值或推断值。

### IMPL-4: Reading Generation Profiles And Validators

重写阅读生成策略，使 prompt 不泄漏、Band 有明显递进、题型结构真实匹配、篇幅多段且接近 IELTS Academic 训练材料。重点在 `OpenAIGenerator.Generate` reading branch 和 `Service.GenerateReadingMaterial` 的请求构造/结果组装。

Acceptance:
- IELTS reading 默认至少多段，低阶和高阶在词数、句法复杂度、抽象度、选项干扰上有明显差异。
- Matching Headings 生成 heading bank + paragraph matches；TFNG 生成 True/False/Not Given；Summary Completion 生成摘要空格/词库或答案；Mixed 组合多个真实题型。
- 题目必须引用或隐含段落证据，不再批量出现通用题如 `What is the main focus of the passage?`。

### IMPL-5: High-Quality Fallback Generators

升级听力和阅读 fallback，使 AI 失败时不会产生大量雷同、低质量或题型错误内容。重点拆分 `fallbackReadingGeneratedContent`、`fallbackReadingContent`、`fallbackGeneratedContent`，避免继续维护两个阅读 fallback 分支。

Acceptance:
- fallback 按 Band/题型/Section 生成不同结构，且通过 IMPL-1 的卫生校验。
- 阅读 fallback 可生成多段 passage 和对应真实题型，不再只给通用选择题。
- 听力 fallback 可按 Section 输出对应场景、信息密度和题目类型。

### IMPL-6: TTS Retry, Resume, And Audio Status Repair

增强 TTS timeout 处理和阅读音频续跑能力，保留小米 TTS/设计音色优先。重点在 `apps/server/internal/ai/tts.go` 的 retry/backoff 和 `apps/server/internal/service/service.go` 的 reading audio chunk 状态处理。

Acceptance:
- 小米 TLS timeout、临时网络错误、HTTP 429/5xx 使用有限次数退避重试。
- 阅读音频 chunk 失败时保留已成功 chunk，并允许后续续跑修复 PENDING/FAILED。
- `stage-12-reading-2` 这类单篇 PENDING/TIMEOUT 状态有明确修复入口或服务方法。

### IMPL-7: Small-Sample Quality Gate

建立扩容前验收流程，先生成 5 篇听力 + 5 篇阅读，自动检查后交给人工确认。该任务不批量扩容，只产出质量报告和可复跑脚本/测试。

Acceptance:
- 自动报告覆盖 prompt 泄漏、英文空格、词数/段落数、题型结构、Band 差异、音频状态。
- 小样本配置覆盖至少 Band 5.0、6.0、7.0 和多个 Section/QuestionType。
- `docs/test-report.md` 或同类文档记录验收结果、剩余风险和是否允许进入 300 篇扩容。

## 4. Implementation Strategy

Recommended execution: phased.

Phase A should execute IMPL-1 first, because所有后续生成任务都依赖基础卫生校验。IMPL-2 可以在 IMPL-1 后独立推进听力；IMPL-3 先处理阅读结构兼容，再由 IMPL-4 接入真实题型生成；IMPL-5 在 IMPL-2/3/4 后统一对 fallback 对齐；IMPL-6 可与 IMPL-4/5 并行；IMPL-7 最后作为质量门禁。

Dependency chain:
1. IMPL-1
2. IMPL-2 and IMPL-3
3. IMPL-4
4. IMPL-5 and IMPL-6
5. IMPL-7

## 5. Risk Assessment

Risk: Reading question model changes may ripple through GraphQL and client pages.
Mitigation: Use additive fields and legacy-compatible JSON shape; keep existing `question/options/answerKey` readable.

Risk: Prompt tightening may increase model failure rate.
Mitigation: Pair stricter validators with improved fallback and clear generation errors.

Risk: Metadata migrations can break existing deployments if not additive.
Mitigation: New migration only adds nullable/default columns; SQLite startup should tolerate existing local DBs.

Risk: TTS retries can make requests slower or costly.
Mitigation: Bound attempts, retry only transient errors, and record per-material failure reasons.

Risk: No existing Go tests were found.
Mitigation: Start with focused unit tests around pure helpers and fake generator/TTS service flows before adding broader integration checks.

## 6. Verification Plan

- Run `go test ./...` from `apps/server` after backend changes.
- Run `npm run typecheck --workspace apps/client` after client type/query changes.
- Run `npm run lint --workspace apps/client` if UI/API files are changed.
- Run the small-sample quality gate from IMPL-7 before any 300-item expansion.

## 7. References

- `apps/server/internal/ai/generator.go:124` difficulty only affects quiz count.
- `apps/server/internal/ai/generator.go:148` main generation entry point.
- `apps/server/internal/service/service.go:525` theater generation orchestration.
- `apps/server/internal/service/service.go:906` reading material generation orchestration.
- `apps/server/internal/service/service.go:1018` low-quality reading fallback branch.
- `apps/server/internal/service/service.go:1304` reading audio generation.
- `apps/server/internal/service/service.go:1681` general fallback generator.
- `apps/server/internal/ai/tts.go:326` Xiaomi TTS request and retry loop.
- `apps/server/internal/domain/models.go:140` ReadingMaterial model lacks explicit requested metadata.
- `apps/server/internal/graph/schema.go:208` GraphQL ReadingMaterial fields.
