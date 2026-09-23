"""清理已核对的口语主题污染；默认预览，保留题干、ID、来源及音频。"""
import argparse
from collections import Counter
from contextlib import closing
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import shutil
import sqlite3
import tempfile


def clean_topic(topic):
    # 只处理已确认的推广后缀，不删除合法中文主题，也不猜补英文题干。
    topic = re.sub(
        r'\s+abckg525\s+如果此素材不是从此微信购买，其他买家均为转卖，\s*$',
        '', topic,
    )
    return {'客服困难终成功': '克服困难终成功'}.get(topic, topic)


def clean_items(items):
    changes = []
    for item in items:
        before = item.get('topic', '')
        after = clean_topic(before)
        if after != before:
            changes.append({'id': item['id'], 'field': 'topic', 'before': before, 'after': after})
            item['topic'] = after
    return changes


def clean_bank(path, apply=False):
    path = Path(path).resolve(strict=True)
    lock = path.with_name(path.name + '.lock')
    # 与导入/音频生成共用锁，避免清理覆盖刚写入的音频。
    with lock.open('x'):
        pass
    try:
        bank = json.loads(path.read_text(encoding='utf-8'))
        if bank.get('version') != 1 or not bank.get('items'):
            raise ValueError('题库版本无效或为空')
        ids = [item['id'] for item in bank['items']]
        if len(ids) != len(set(ids)):
            raise ValueError('题库包含重复 ID，请先人工核对')
        changes = clean_items(bank['items'])
        catalog = path.with_name('catalog.db')
        updates = []
        if catalog.exists():
            with closing(sqlite3.connect(catalog.as_uri() + '?mode=ro', uri=True)) as db:
                for key, raw in db.execute('SELECT id,payload FROM speaking_items'):
                    item = json.loads(raw)
                    if clean_items([item]):
                        updates.append((json.dumps(item, ensure_ascii=False), key))
        report = {
            'items': len(ids), 'byPart': dict(Counter(item['part'] for item in bank['items'])),
            'changes': changes, 'catalogTopicsChanged': len(updates), 'applied': False,
        }
        if not apply or not (changes or updates):
            return report
        backup = Path(tempfile.mkdtemp(prefix='topic-cleanup-', dir=path.parent))
        shutil.copy2(path, backup / path.name)
        if catalog.exists():
            with closing(sqlite3.connect(catalog)) as db, closing(sqlite3.connect(backup / catalog.name)) as saved:
                db.backup(saved)
        report['backup'] = str(backup)
        report['applied'] = True
        report['timestamp'] = datetime.now(timezone.utc).isoformat()
        # 报告与原件放在一起；即使后续写入失败也能按备份恢复。
        (backup / 'cleanup-report.json').write_text(
            json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8',
        )
        staged = backup / 'cleaned-speaking.json'
        staged.write_text(json.dumps(bank, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
        try:
            if updates:
                with closing(sqlite3.connect(catalog)) as db:
                    with db:
                        db.executemany('UPDATE speaking_items SET payload=? WHERE id=?', updates)
                        if changes:
                            staged.replace(path)
            elif changes:
                staged.replace(path)
        except Exception:
            shutil.copy2(backup / path.name, path)
            raise
        return report
    finally:
        lock.unlink()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bank', type=Path, required=True)
    parser.add_argument('--apply', action='store_true', help='备份后执行；不指定时只预览')
    args = parser.parse_args()
    print(json.dumps(clean_bank(args.bank, args.apply), ensure_ascii=False, indent=2))
