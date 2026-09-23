# 已授权 IELTS 基础题库

## 本地数据和来源

用户确认拥有资料使用授权。原始目录只读，不把题库内容自动提交到 GitHub。
目录资产清单包含所有文件，但“已登记”不代表“已解析/可用于考试”。

首批解析三个经过格式核对的 PDF：2022 年 5–8 月 Part 1、Part 2 题库，以及
2023 年 1–4 月口语保留题库。原文、来源文件名、页码、源文件 SHA-256 均保留。
不包含参考答案，不生成假考生回答；历史题库不宣称为当季题或官方授权产品。

生成文件（默认已忽略 Git）：

- `apps/server/data/question-bank/catalog.db`：资料清单和提取题目的管理数据库。
- `apps/server/data/question-bank/speaking.json`：后端实际读取的题库发布快照。
- `import-report.json`：数量及来源报告。
- `review-required.json`：解析校验未通过的记录。

管理数据库的音频状态在再次导入时从发布快照同步；应用以 JSON 快照为准。
未解析的 Word、扫描 PDF、压缩包、听力/阅读完整试卷和答案不自动投入考试。
不通过 PDF 页码或相近文件名猜测听力录音的对应关系。

## 导入

项目根目录执行，Python 依赖 `pdfplumber`：

```powershell
python scripts/import_ielts_bank.py --source "F:/BaiduNetdiskDownload/雅思" --output apps/server/data/question-bank
python -m unittest discover -s scripts -p test_import_ielts_bank.py
```

内容哈希去重，重复运行保留已生成音频。解析格式不匹配的其他文件仅登记清单，
不通过 AI 猜补。导入器与音频工具使用同一排他锁，不能同时修改快照；
进程崩溃留下 `.lock` 时，须确认无导入/生成进程后再手动移除锁。

## 主题清理

无需连接原始资料盘即可清理已有题库。先预览，确认后加 `--apply`：

```powershell
python scripts/clean_speaking_bank.py --bank apps/server/data/question-bank/speaking.json
python scripts/clean_speaking_bank.py --bank apps/server/data/question-bank/speaking.json --apply
python -m unittest discover -s scripts -p test_clean_speaking_bank.py
```

目前仅修正已核对的 `Sitting down` 推广水印后缀，以及“客服困难终成功”主题错字。
合法中文主题不删除，英文题干不猜补；题目 ID、来源、Part 2/3 配套关系与音频保持原样。
同目录 `catalog.db` 存在时同步修正其主题，不覆盖管理库自身的音频状态。
执行前在题库目录创建 `topic-cleanup-*` 备份目录，保存原始 JSON、SQLite 一致性备份和
`cleanup-report.json` 逐项记录。工具与导入/音频工具共用排他锁，重复执行不会产生重复改动。
导入器复用相同主题清理规则，避免再次导入带回已修正的问题。

2026-09-15 清理结果：4 条推广水印、5 条主题错字；保留 350 条题目和 350 个音频链接。
已用真实本地快照通过 100 次配套抽题检查。清理不代表原资料英文内容已完成专家校对。
后端在开始新会话时读取 JSON，因此数据清理无需重启；已有会话仍保留原题目快照。

## 接入 Speaking

后端从工作目录相对路径 `data/question-bank/speaking.json` 自动加载。
可在后端环境配置绝对路径 `SPEAKING_BANK_PATH`。新会话按两个 Part 1 主题各选三题，
随机选一个 Part 2 题卡及其原始配套 Part 3 追问。未配套的题卡留在题库，不随意拼接。
会话保存完整题目快照；重新导入不会替换正在进行的会话。
默认路径不存在时保留旧版内置练习；显式配置无效、JSON 损坏或题库不完整时报错，
不偷偷改回内置内容。已启动的旧后端需要重启才会加载新代码。

## 音频

在 `apps/server` 工作目录运行：

```powershell
go run ./cmd/bank-audio -bank data/question-bank/speaking.json -limit 100
```

读取当前后端 `.env` 和数据库中的 TTS 配置，与服务启动时的优先级一致。
默认一批三条，上限一百条，串行调用、跳过已有音频、每条成功后保存，失败即停。
使用现有 TTS 调度和媒体落盘逻辑；需要 FFmpeg，音频解码通过才关联到题目。
这是管理员离线预生成，不扣某个用户的积分，但会消耗 TTS 厂商额度。
当前工具只接受可本地持久化的内联音频，拒绝把临时远程 URL 当成永久资源。
生成的是考官题目朗读，标记 `GENERATED_TTS`，不是原始考试录音或考生示范回答。
FFmpeg 解码验收不等于发音、语义或 IELTS 难度校准。

已有音频可用精确映射导入，映射 JSON 的格式为：

```json
[{"questionId":"题库中的64位ID", "file":"相对原始资料目录的音频路径.mp3"}]
```

```powershell
python scripts/link_ielts_audio.py --source "F:/BaiduNetdiskDownload/雅思" --bank apps/server/data/question-bank/speaking.json --media apps/server/media --mapping 已核对音频映射.json
```

工具校验目录边界、重复 ID 和音频解码，将媒体复制到服务器目录，标记为
`SOURCE_AUDIO`，保留 `audioSource`。预生成工具跳过这些题目。
不能将 F 盘路径直接发给浏览器。当前资料还没有经过人工确认的映射，未执行此工具导入原始音频。

## 服务器部署

单独传输 `speaking.json` 和音频文件，挂载到容器的数据目录和 `MEDIA_DIR`，
设置 `SPEAKING_BANK_PATH` 指向容器内 JSON 路径。无需在线上安装 PDF 提取依赖。
数据库迁移 012 提供 Speaking 会话表。应用安装包只访问服务端 GraphQL 和媒体路径，
不内置教材。此轮不会自动推送、发布或重启线上服务。

## 验证

```powershell
# 在 apps/server 中执行；不向 CI 提交原始题库
$env:TEST_SPEAKING_BANK=(Resolve-Path data/question-bank/speaking.json).Path
go test ./...
```

包含配套抽题、重复 ID、异常音频路径和 GraphQL 鉴权/来源字段校验。
