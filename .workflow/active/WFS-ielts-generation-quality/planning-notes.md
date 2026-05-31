# Planning Notes

## User Intent

GOAL: 修复 IELTS 听力与阅读生成质量问题，先生成可验收的小样本，再决定是否扩到 300 篇。

SCOPE: 后端内容生成链路、fallback 模板、难度与题型控制、音频生成稳定性、课程元数据、质量验收流程。当前阶段仅规划，不执行实现。

CONTEXT: 用户已确认优先级是“先修生成质量”。现有问题覆盖听力英文空格丢失、听力难度控制弱、Section 题型不明确、阅读 prompt 泄漏、阅读题型不匹配、阅读难度无递进、篇幅结构不真实、题目通用、fallback 质量低、TTS timeout、元数据不足、批量生成前缺少验收。

## Requirements

### P0

1. 修复听力英文空格丢失，避免字幕、题目、TTS 文本出现 `Goodafternoon,BrookdaleLanguageCentre` 这类不可读内容。
2. 强化听力 difficulty/Band 控制，覆盖语速感、句长、词汇、paraphrase、干扰项密度，而不是只影响题目数。
3. 明确听力 Section 1/2/3/4 的任务类型：表格/数字/拼写、地图/说明、观点匹配、笔记摘要。
4. 修复阅读 prompt 泄漏，正文不得包含 `Task design`、`Create an IELTS Academic reading drill` 等提示词。
5. 修复阅读题型标签与实际题目结构不匹配。
6. 强化阅读 Band 递进，控制词汇、句法、段落复杂度、干扰项强度。
7. 提升阅读 fallback 模板，AI 失败时也按题型和 Band 生成合格结构。
8. 增加批量生成前质量验收，先生成 5 篇听力 + 5 篇阅读小样本。

### P1

1. 阅读至少多段结构，高阶更长、更抽象、更密集。
2. 阅读题目基于段落证据、定位、同义替换、细节陷阱生成，避免通用题。
3. 加强小米 TTS TLS timeout 的重试、续跑、状态修复。
4. 课程清单显式记录 `band` / `stage` / `section` / `skill_focus` / `question_type` / `scenario_family`。

## Planning Constraints

- 保持向后兼容，不破坏现有课程、音频、客户端读取逻辑。
- 优先修生成质量，不直接扩容到 300 篇。
- 计划完成后应能交给 `$workflow-execute --session WFS-ielts-generation-quality` 执行。

## Context Findings

- Critical backend files: `apps/server/internal/ai/generator.go`, `apps/server/internal/service/service.go`, `apps/server/internal/ai/tts.go`.
- Persistence/API files for metadata: `apps/server/internal/domain/models.go`, `apps/server/internal/graph/schema.go`, `apps/server/internal/store/sqlite_store.go`, `apps/server/internal/store/postgres_store.go`, `apps/server/migrations/`.
- Client inspection files: `apps/client/src/api.ts`, `apps/client/src/types.ts`, reading pages under `apps/client/src/pages/`.
- Conflict risk: medium, because metadata touches database/API/client, while generation and fallback share large backend files.
- Important constraint: `apps/server/internal/ai/prompt_templates.jsonl` exists but current `OpenAIGenerator.Generate` still constructs main reading/dialogue prompts inline, so prompt changes must target actual call sites.
