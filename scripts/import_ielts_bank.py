"""离线提取已授权 PDF 口语题库；原件只读，未知格式不猜题目/答案。"""
import argparse
import hashlib
import json
import re
import sqlite3
from collections import Counter
from pathlib import Path

import pdfplumber
from clean_speaking_bank import clean_topic


def digest(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()


def extract(path):
    with pdfplumber.open(path) as pdf:
        return [(number, line.strip()) for number, page in enumerate(pdf.pages, 1)
                for line in (page.extract_text(x_tolerance=1) or '').splitlines()]


def parse(lines, source, start_page=1):
    items, rejected = [], []
    part, topic, group, pending, first_page = 1, '', '', '', 0
    cue = []

    def flush_question():
        nonlocal pending
        if pending:
            add(part, pending, first_page)
            pending = ''

    def add(p, text, page):
        text = re.sub(r'\s+', ' ', text).strip()
        valid = (len(text.split()) >= 4 and len(text) <= 1800
                 and not re.search(r'[\u4e00-\u9fff]|\ufffd', text))
        valid = valid and ((p == 2 and 'You should say:' in text and len(text.split()) >= 20
                           and re.search(r'\bAnd\b', text)) or (p != 2 and text.endswith('?')))
        if not valid:
            rejected.append({'source': source, 'page': page, 'text': text, 'reason': 'incomplete_or_contaminated'})
            return
        key = digest(f'{p}:' + text.lower())
        items.append({'id': key, 'part': p, 'topic': topic, 'group': group if p > 1 else '',
                      'question': text if p != 2 else '', 'cueCard': text if p == 2 else '',
                      'preparationSec': 60 if p == 2 else 0, 'answerSec': 120 if p == 2 else 45,
                      'source': source, 'sourcePage': page, 'audioUrl': '', 'audioStatus': 'MISSING'})

    cue_page = 0
    for page, original in lines:
        if page < start_page:
            continue
        line = original.replace('？', '?').replace('：', ':')
        if not line or re.fullmatch(r'\d+\s*/\s*\d+', line):
            continue
        if re.match(r'^Part\s*2', line, re.I):
            flush_question()
            part = 2
            continue
        if re.fullmatch(r'P[123]\.?', line, re.I):
            flush_question()
            if line.upper().startswith('P3'):
                if cue:
                    add(2, ' '.join(cue), cue_page)
                    cue = []
                part = 3
            continue
        heading = re.match(r'^\d+\.\s*(.+)', line)
        if heading and not heading[1].startswith('Describe '):
            flush_question()
            topic = clean_topic(heading[1])
            continue
        description = re.match(r'^(?:\d+\.\s*)?(Describe .+)', line)
        if description:
            flush_question()
            if cue:
                add(2, ' '.join(cue), cue_page)
            part, cue_page = 2, page
            group = digest(source + ':' + description[1].lower())
            cue = [description[1]]
            continue
        if re.search(r'[\u4e00-\u9fff]', line):
            # 已核对该 PDF 第四页：提示语后紧跟中文推广水印；只保留其原有英文前缀。
            if (source == '2022年5-8月雅思口语Part2题库.pdf' and page == 4
                    and re.match(r'^(What it is|How it was broken)\s', line)):
                line = re.split(r'[\u4e00-\u9fff]', line, maxsplit=1)[0].strip()
            else:
                continue
        if part == 2 and cue:
            if line == 'You should say:' and line in cue:
                continue
            cue.append(line)
            continue
        if part in (1, 3):
            if line == 'Why?' and items and items[-1]['part'] == part:
                items[-1]['question'] += ' Why?'
                items[-1]['id'] = digest(f'{part}:' + items[-1]['question'].lower())
                continue
            if re.match(r'^(Do|Does|Did|What|When|Where|Why|Who|How|Are|Is|Have|Has|Would|Will|Can|Should|Which|Were|Could)\b', line) and pending.endswith('?'):
                flush_question()
            if not pending:
                if not re.match(r'^(Do|Does|Did|What|When|Where|Why|Who|How|Are|Is|Have|Has|Would|Will|Can|Should|Which|Were|Could)\b', line):
                    continue
                first_page = page
            pending = (pending + ' ' + line).strip()
            # 保留同一行的 Why?，不把范文或下一题并入。
            if pending.endswith('?'):
                flush_question()
    flush_question()
    if cue:
        add(2, ' '.join(cue), cue_page)
    return items, rejected


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    root, out = args.source.resolve(), args.output.resolve()
    if not root.is_dir() or out == root or root in out.parents:
        parser.error('输出须在原始资料目录之外')
    out.mkdir(parents=True, exist_ok=True)
    lock = out / 'speaking.json.lock'
    with lock.open('x'):
        pass
    try:
        import_bundle(root, out)
    finally:
        lock.unlink()


def import_bundle(root, out):
    # 格式已核对的白名单；未知 PDF、扫描件与 Word 等只进入资产清单。
    selected = [('2022年5-8月雅思口语Part1题库.pdf', 1),
                ('2022年5-8月雅思口语Part2题库.pdf', 1),
                ('2023年1月-4月雅思口语保留题库(1)(1).pdf', 3)]
    inventory = [{'path': str(p.relative_to(root)), 'bytes': p.stat().st_size,
                  'extension': p.suffix.lower(), 'status': 'UNREVIEWED'}
                 for p in sorted(root.rglob('*')) if p.is_file()]
    all_items, rejected, sources = [], [], []
    for name, start in selected:
        path = root / name
        if not path.is_file():
            rejected.append({'source': name, 'reason': 'source_missing'})
            continue
        items, failures = parse(extract(path), name, start)
        sha = hashlib.sha256(path.read_bytes()).hexdigest()
        sources.append({'file': name, 'sha256': sha, 'rights': 'USER_AUTHORIZED', 'items': len(items)})
        all_items.extend(items)
        rejected.extend(failures)
    unique = {}
    for item in all_items:
        if item['id'] in unique:
            unique[item['id']]['sources'].append({'file': item['source'], 'page': item['sourcePage']})
        else:
            item['sources'] = [{'file': item['source'], 'page': item['sourcePage']}]
            unique[item['id']] = item
    items = list(unique.values())
    # 再次运行保留已生成/已人工匹配音频。
    bank_path = out / 'speaking.json'
    if bank_path.exists():
        prior = {x['id']: x for x in json.loads(bank_path.read_text(encoding='utf-8'))['items']}
        for item in items:
            old = prior.get(item['id'], {})
            item['audioUrl'] = old.get('audioUrl', '')
            item['audioStatus'] = old.get('audioStatus', 'MISSING')
            if old.get('audioSource'):
                item['audioSource'] = old['audioSource']
    bank = {'version': 1, 'sources': sources, 'items': items}
    groups = Counter(x['group'] for x in items if x['part'] == 3)
    report = {'assets': len(inventory), 'extensions': dict(Counter(x['extension'] for x in inventory)),
              'imported': len(items), 'byPart': dict(Counter(x['part'] for x in items)),
              'duplicates': len(all_items)-len(items), 'rejected': len(rejected),
              'pairedCueCards': sum(x['part'] == 2 and groups[x['group']] >= 3 for x in items),
              'audioReady': sum(bool(x['audioUrl']) for x in items),
              'audioMatched': sum(x['audioStatus'] == 'SOURCE_AUDIO' for x in items),
              'audioGenerated': sum(x['audioStatus'] == 'GENERATED_TTS' for x in items), 'sources': sources}
    with sqlite3.connect(out / 'catalog.db') as db:
        db.execute('CREATE TABLE IF NOT EXISTS assets(path TEXT PRIMARY KEY, metadata TEXT NOT NULL)')
        db.execute('CREATE TABLE IF NOT EXISTS speaking_items(id TEXT PRIMARY KEY, part INTEGER NOT NULL, payload TEXT NOT NULL)')
        db.executemany('INSERT INTO assets VALUES(?,?) ON CONFLICT(path) DO UPDATE SET metadata=excluded.metadata',
                       [(x['path'], json.dumps(x, ensure_ascii=False)) for x in inventory])
        db.executemany('INSERT INTO speaking_items VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload,part=excluded.part',
                       [(x['id'], x['part'], json.dumps(x, ensure_ascii=False)) for x in items])
        # 仅清理本批三个来源之前提取的旧版本，保留其他批次。
        old_rows = db.execute('SELECT id,payload FROM speaking_items').fetchall()
        managed = {name for name, _ in selected}
        obsolete = [(key,) for key, raw in old_rows
                    if json.loads(raw).get('source') in managed and key not in unique]
        db.executemany('DELETE FROM speaking_items WHERE id=?', obsolete)
    for name, payload in [('speaking.json', bank), ('import-report.json', report), ('review-required.json', rejected)]:
        temp = out / (name + '.tmp')
        temp.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding='utf-8')
        temp.replace(out / name)
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
