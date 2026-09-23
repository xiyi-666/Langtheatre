"""将人工核对的原始音频关联到题目，不能通过文件名猜对应关系。"""
import argparse
import hashlib
import json
import re
import shutil
import subprocess
from pathlib import Path


def resolve_audio(root, relative):
    file = (root / relative).resolve()
    if root not in file.parents or not file.is_file():
        raise ValueError('音频路径不存在或不在指定原始资料目录内')
    if file.suffix.lower() not in {'.mp3', '.wav', '.m4a', '.ogg'}:
        raise ValueError('不支持的音频格式')
    if file.stat().st_size > 200 * 1024 * 1024:
        raise ValueError('请先为题目准确分段，单文件上限 200MB')
    return file


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--bank', type=Path, required=True)
    p.add_argument('--media', type=Path, required=True)
    p.add_argument('--mapping', type=Path, required=True)
    args = p.parse_args()
    root, bank_file, media = args.source.resolve(), args.bank.resolve(), args.media.resolve()
    if media == root or root in media.parents or bank_file == root or root in bank_file.parents:
        p.error('题库和媒体输出必须在原始资料目录之外')
    if not shutil.which('ffmpeg'):
        p.error('缺少 ffmpeg，无法校验音频')
    lock = bank_file.with_name(bank_file.name + '.lock')
    with lock.open('x'):
        pass
    try:
        bank = json.loads(bank_file.read_text(encoding='utf-8'))
        items = {item['id']: item for item in bank['items']}
        mapping = json.loads(args.mapping.read_text(encoding='utf-8'))
        verified = []
        seen = set()
        for row in mapping:
            key = row['questionId']
            if key not in items or not re.fullmatch('[0-9a-f]{64}', key) or key in seen:
                raise ValueError('题目标识无效、重复或未导入')
            seen.add(key)
            file = resolve_audio(root, row['file'])
            subprocess.run(['ffmpeg', '-v', 'error', '-xerror', '-i', str(file), '-f', 'null', '-'],
                           check=True, capture_output=True, timeout=120)
            verified.append((key, file))
        # 全部映射检查通过后再复制，原始文件保持只读。
        for key, file in verified:
            with file.open('rb') as stream:
                sha = hashlib.file_digest(stream, 'sha256').hexdigest()
            rel = Path('tts/question-bank') / key / (sha + file.suffix.lower())
            destination = media / rel
            if media not in destination.resolve().parents:
                raise ValueError('媒体目录存在越界符号链接，停止复制')
            destination.parent.mkdir(parents=True, exist_ok=True)
            if not destination.exists():
                shutil.copyfile(file, destination)
            items[key]['audioUrl'] = '/media/' + rel.as_posix()
            items[key]['audioStatus'] = 'SOURCE_AUDIO'
            items[key]['audioSource'] = str(file.relative_to(root))
        temp = bank_file.with_name(bank_file.name + '.tmp')
        temp.write_text(json.dumps(bank, ensure_ascii=False, indent=2), encoding='utf-8')
        temp.replace(bank_file)
        print(f'已关联 {len(verified)} 条经人工确认对应关系的原始音频')
    finally:
        lock.unlink()


if __name__ == '__main__':
    main()
