# 听力原文专项质检（2026-09-16）

后续已完成一轮[全新自动原文抽检](listening-automatic-text-sampling.md)：4 份生成成功、1 份超时。4 份语言表达均通过，但存在逻辑与技术表述问题；下述两篇修订稿通过不能代替自动生成稳定性结论。

## 7.5、8.0 修订复验：已通过

按用户要求修订原文后，用同一文本审核标准重新进行独立模型审核，不发送旧审核结论、目标档位或题目难度配比。**两份修订稿均通过语法、自然度、连贯性和体裁检查，issues 为空。** 这是具体修订稿的文本质量通过，不代表所有后续自动生成内容都已通过，也不是官方难度等值或音频验收。

| 修订稿 | 主要修订 | 结果 |
| --- | --- | --- |
| 7.5：Marinthos 测年项目讨论 | 区分样本年代和建筑事件；说明侵入、重沉积、光信号重置等条件；前后统一结论，保留三人 24 轮对话 | 四项均 PASS，约 19 秒，无遗留问题 |
| 8.0：字母文字演变讲座 | 修复缺词和残句；明确希腊元音字母的重新分配；去掉没有依据的演变动机；将例子、证据与推测分开，保留单人 8 段讲座 | 四项均 PASS，约 9 秒，无遗留问题 |

修订原文保存在 `.workflow/ielts-runtime/text-quality-20260916/revised/part3-target7.5.txt` 和 `part4-target8.0.txt`。按空白分词并去掉说话人标签约为 766/734 词，均在 650–850 词范围内。

原始复验报告：`.workflow/ielts-runtime/text-quality-20260916/run-1789547682199/`，报告保留完整修订文本及 SHA-256：

- 7.5：`e69ef5aa8e5e32d9e2a386ae92607569ba487ab8fe2af1355bbf58badb451c41`
- 8.0：`cf9a1dcfb1749b2d435a29bc4134cdbddd6c6e8498bbb2ac09c238e4ca62d4ef`

代码同步增加了学术原文的写作约束：区分直接测量和推断事件、说明适用条件、不虚构历史动机、保持指代和总结一致。冻结前审核也增加对应检查，并避免把可接受并列句误判为语法错误。没有修改难度配比或伪造审核结论。原稿和此前失败报告继续保留，未替换数据库中的旧题目、证据或录音；它们与新文本的对应关系未重新验收，不可直接混用。

## 初次检查记录（修订前，保留）

## 范围与结论

按用户澄清，本次只检验已有 7.0、7.5、8.0 样本的听力原文：语法、自然表达、连贯性、角色/场景一致性及明显误导性表述。不生成题目，不检查基础题/挑战题配比，不生成音频，不测语速，也不认证官方 Band 等值。三个目标标签只用于定位样本，不提供给审核模型。

三次真实文本审核均成功返回，分别约 51、22、28 秒，无请求超时。使用现有配置模型 `gpt-5.6-terra`，串行调用，测试请求直连网关，未改持久模型配置。三个样本不能代表该档位所有未来生成内容。

| 样本 | 语言质量 | 仍需处理的问题 | 可用结论 |
| --- | --- | --- | --- |
| Part 3 / 7.0 | 对话自然、人物和推理链清楚，未发现明确缺词或破碎句 | 同位素筛查能否判断燃料来源/旧木炭搬运的说法缺乏方法说明；沉积年代对构筑物的时间约束应说明方向与条件 | 基本语言可用，技术解释需修订或核实，不能无保留批准全文 |
| Part 3 / 7.5 | 语法、自然度、连贯性、对话体裁均通过 | 大麦粒的测年结果不能自动等同于房间最后使用时间；需说明样品与沉积/使用事件的关联条件 | 语言质量通过，技术解释需要限定 |
| Part 4 / 8.0 | 有明确语法缺词、不自然表达及含义不完整的句子 | 另有希腊元音字母、拉丁 C/G 演变的表述过满或依据不清 | 旧稿不合格，需修订后复验 |

## 可复核的具体问题

### 7.0：方法解释的适用范围

原句：`If water carried old charcoal into the hearth, isotope results could warn us that the fuel was not local, even though they would not date the charcoal directly.`

没有说明具体同位素项目、参照数据及方法能回答的问题，不能据此保证识别搬运或燃料产地。建议明确此处为待验证的学生假设，并由导师指出需要先确认分析方法的能力与样品关联。

原句：`So luminescence can provide a terminus for both features, but it cannot independently prove they were erected together.`

应说明是在地层关系可靠、样品符合测年条件时提供某一方向的时间约束，不直接给出墙基和木桩的共同建造年代。具体考古方法的准确性仍需专业来源支持。

审核器另将末句 `I will request ... today, Mira can prepare ..., and Owen can arrange ...` 标记为逗号拼接句。复核认为它也可作为三个独立分句的并列列举理解，不能仅凭这个标签判定明确语法错误；可拆句提升听感，但属于可选编辑。保留审核器原始结论，不把它包装成确定事实。

### 7.5：样品年代与事件年代

原句：`So the grains would date the final use of that room more closely than the platform.`

建议将“直接测定最后使用时间”改为“在样品非侵入、沉积关联可靠等条件下，为房间晚期使用或沉积提供时间约束”。后文再次使用 `the grain date gives the room's last use`，修订时应一致处理，避免只改一处。

### 8.0：明确的语言缺陷

| 原文 | 最小语言修正 |
| --- | --- |
| `has page made easier to navigate?` | `has the page been made easier to navigate?` |
| `retain an convention` | `retain a convention` |
| `We should trace users faced problems` | `We should trace what problems users faced` |

`A response, retaining borrowed values and inventing vowel marks, would have produced a system` 缺乏清楚的对比与指代，需重写该句表达完整意思。

`Greek communities did not add vowels to a Phoenician template` 的绝对否定易误导，应明确重新分配既有符号为元音字母这一含义。拉丁字母分化归因于 `educational habits` 的因果解释也需要可靠依据。首音原则相关句子本来已经用了 `Evidence ... suggests`，审核器进一步要求谨慎不能作为其确定错误的证明。

## 证据与边界

输入：`.workflow/ielts-runtime/quality-improvement-20260916-final/listening-difficulty-2930803811/` 下的 `part3-target7.0.json`、`part3-target7.5.json`、`part4-target8.0.json`。

本次逐份原始审核：`.workflow/ielts-runtime/text-quality-20260916/run-1789547122306/`。报告记录源文本 SHA-256、模型、逐维度判断、原文引文与耗时；引文须逐字存在于输入。没有保存密钥、认证头或供应商原始错误。

独立模型审核属于辅助质检，可能误报；以上最终意见同时依据逐段阅读和引文核对。没有完成考古学或文字史的专业事实认证。本次未重写原样本、修改出题门槛或上线内容。后续针对所列原句修订，并仅复验文本即可；不需要为证明语言质量重新生成整套题。
