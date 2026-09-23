# 雅思听力本地题库导入

`scripts/import_ielts_listening_bank.py` 用于扫描本地授权资料并生成候选清单。它不会把资料直接发布到学员可见的听力训练列表，只有同时满足以下条件的记录才允许进入正式题库：

1. 题目文本经过人工检查，修复扫描/OCR 字符错误；
2. 官方答案表已确认，且每道题都能从原文定位证据；
3. 音频与相同书册、相同 Test、相同 Section 明确对应，并通过播放校验；
4. 难度审核通过，且内容哈希与审核记录一致。

扫描本地资料：

```powershell
python scripts/import_ielts_listening_bank.py `
  --source 'F:\BaiduNetdiskDownload\雅思\06.雅思听力' `
  --output '.workflow\ielts-runtime\listening-bank-candidates.json'
```

候选清单包含题本来源、PDF SHA-256、Section 文本、音频路径和音频 SHA-256。`.workflow/` 是本地审核产物，不应提交到 GitHub，也不应把版权音频复制进仓库。

当前扫描结果：剑桥雅思 4 的 Test 1–4 题目 PDF 可以读取部分文本，但音频以 `CassetteSide` 形式存在，不能与 Section 直接一一对应；PDF 也没有可确认的完整官方答案表。因此这批记录保持 `REVIEW_REQUIRED`，不会被前端作为正式试卷展示。
