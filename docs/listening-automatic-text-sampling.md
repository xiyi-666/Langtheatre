# 自动生成听力原文抽检（2026-09-16）

## 结论

后续代码优化与同稿复检见 [7.5 / 8.0 原文质量改进](listening-high-band-quality.md)。原始抽检记录保持不变，后续修订不覆盖首次失败结论。

覆盖 6.0、6.5、7.0、7.5、8.0，各预定一份不同主题的新原文。4 份成功生成，1 份请求超时。成功生成的 4 份均通过独立模型的语法、自然度和体裁检查；完整文本的逻辑与技术表述仍有缺陷或需核实内容，**不能据此认定自动原文生成质量已稳定通过**。

本次抽检没有人工改稿，没有换题重抽到成功。7.5 原文按现有生成器自动纠正了一次超长问题，初稿保留。6.5 超时后没有重试该档位，只在另一批次继续尚未尝试的 7.0–8.0。审核者没有收到目标档位、题目配比或作者审批结果。

## 样本结果

| 档位 / Part | 自动生成主题 | 原文词数 | 生成情况 | 文本质检 |
| --- | --- | ---: | --- | --- |
| 6.0 / 1 | 社区运动中心入会咨询 | 753 | 1 次，约 33 秒 | 语法和自然度通过；周二游泳说明会被描述为与周三羽毛球冲突，存在明确日程矛盾 |
| 6.5 / 2 | 未取得原文 | — | 240 秒后请求超时 | 无法评价文本，不能算语言不合格，也不能计为通过 |
| 7.0 / 3 | 目击者记忆实验设计 | 803 | 1 次，约 40 秒 | 语言通过；对 48 小时延迟效果的表述偏确定，需要限定为预期或预试结果 |
| 7.5 / 3 | 纤维食品包装实验设计 | 779 | 初稿 922 词，自动修订后通过长度检查，共约 74 秒 | 语言通过；周五湿润处理后的压缩测试没有明确仪器安排，实验日程有缺口 |
| 8.0 / 4 | 山地冰川形成与运动 | 787 | 1 次，约 47 秒 | 语言通过；冰川坡度的季节性表述需澄清，测量桩数量前后交代不一致 |

以上是小样本内容检查，不用于推断长期准确率、全部 Part/档位组合、官方难度、音频或语速质量。独立模型的完整审批为 0/4；这不等于“4 篇英语都写得差”，因为语法和自然度均为 PASS，拒绝点主要在逻辑和专业解释。审核器存在误报，以下区分复核意见。

## 原句与人工复核

### 6.0：明确日程矛盾

先有 `beginner badminton sessions on Wednesday evenings`，后有 `the Tuesday evening pool orientation`，却说后者 `finishes too late for the seven-fifteen badminton session`，并再次复述 `Tuesday covers swimming but prevents badminton that night`。这些安排不是同一天，不能按时间冲突解释。

### 7.0：不要把审核器的因果解读当作确定错误

`The delay is long enough to reduce simple rehearsal ...` 将一个有条件的实验预期写得过于确定，应说明依据或改为待验证预期。

审核器还将 `We would infer an effect on reconstruction only if their later answers differ` 解读为仅凭回答差异证明记忆重建改变。原句的 `only if` 实际表达必要条件，而非充分条件，因此“直接等同”的批评偏强；不过，补充回答偏差等替代解释仍有助于避免误导。这不是明确语法错误。

### 7.5：日程缺口与有争议的样品分组

`The compression frame is available on Thursday afternoon`，湿润处理安排在周五，最终主结果却涉及压缩变形；原文未明确周五处理后的压缩仪器时段。应补齐安排，或明确湿润组只用于质量、外观和切片观察，不参与所述压缩比较。

审核器认为 `Three ... untreated, three ... pad exposure, and three ... dry heat ... two for sectioning after moisture exposure and one reserve` 数量矛盾。3+3+3+2+1 本身等于 12，可以解释为三件湿润后压缩样品加两件湿润后切片样品；问题是分组目的表达不够清楚，不能直接断言总数算错。

审核器还要求定义“首次永久变形”的卸载/恢复判据。这是实验方法完善建议，不能把简短听力讨论缺少完整实验规程本身一律当作语言不合格。

### 8.0：数量交代与坡度指代

原文先说 `we placed one near the accumulation zone and another lower down`，后有上部桩的测量，再出现 `both lower stakes accelerated`。没有交代新增下部桩，数量描述不一致。这是逐段阅读发现的问题，独立模型本轮没有指出，说明仍不能完全依赖其审批。

`the lower glacier is steepest in early summer` 未明确是冰面坡度的实测变化，还是空间上坡度较陡的下部区域。审核器提示该句可能误导，应明确所指；不能据此断言所有冰川表面坡度都不随季节变化。

## 方法、证据与边界

生成使用本地实际配置的 `gpt-5.6-terra`，调用现有 `generateMockExamSourceWithPrompt`，沿用 `mockExamSourcePrompt`、`listeningDifficultyGuidance` 和既有长度/角色/正文校验。只在请求前选择不同主题，不按成稿质量挑选样本。该抽检停在原文生成阶段，**没有执行后续 `prepareListeningSource` 的生产原文审核及自动语义修订**，因此结论针对生成阶段的自动稿，不代表这些问题稿已经被线上完整流程放行。

生成记录及原始输出：

- `.workflow/ielts-runtime/automatic-text-sampling-20260916/fresh-text-980039063/`（6.0、6.5）
- `.workflow/ielts-runtime/automatic-text-sampling-20260916/fresh-text-1687051642/`（7.0、7.5、8.0）

独立文本审核：`.workflow/ielts-runtime/text-quality-20260916/run-1789549049410/`。每份保存原文、SHA-256、逐项引文、审批和请求耗时。审核耗时分别约 12、31、45、20 秒；6.5 无原文，不调用审核。

抽检入口为 `apps/server/internal/ai/listening_text_live_test.go` 的 `TestLiveListeningTextSamples`。默认跳过，只有显式开启 `LISTENING_TEXT_LIVE=1` 并提供绝对私有目录 `LISTENING_TEXT_OUTPUT_DIR` 才调用真实模型。只读模型配置，不写业务数据库，不生成题目/TTS，不保存认证信息。测试进程使用网关直连，未修改持久代理或模型配置。

后续应针对同一份失败文本检查生产原文审核能否识别并修正问题，再评价自动流程最终输出；不应重新手工润色几个样本就宣布整体通过。
