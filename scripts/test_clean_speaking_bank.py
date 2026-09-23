import copy
from contextlib import closing
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest

from clean_speaking_bank import clean_bank, clean_topic


class SpeakingCleanupTests(unittest.TestCase):
    def test_preserves_legitimate_chinese_and_english_topics(self):
        for topic in ['时尚人士', 'Advertisements', 'Work or studies', '']:
            self.assertEqual(clean_topic(topic), topic)
        self.assertEqual(clean_topic('客服困难终成功'), '克服困难终成功')

    def test_preview_backup_catalog_and_idempotence(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / 'speaking.json'
            item = {'id': 'q1', 'part': 1,
                    'topic': 'Sitting down abckg525 如果此素材不是从此微信购买，其他买家均为转卖，',
                    'question': 'What is your favorite place to sit?', 'group': '',
                    'source': 'fixture.pdf', 'sourcePage': 1,
                    'audioUrl': '/media/existing.mp3', 'audioStatus': 'GENERATED_TTS'}
            bank = {'version': 1, 'sources': [{'file': 'fixture.pdf'}], 'items': [item]}
            raw = json.dumps(bank, ensure_ascii=False)
            path.write_text(raw, encoding='utf-8')
            catalog_item = dict(item, audioUrl='', audioStatus='MISSING')
            with closing(sqlite3.connect(Path(folder) / 'catalog.db')) as db:
                db.execute('CREATE TABLE speaking_items(id TEXT PRIMARY KEY, payload TEXT)')
                db.execute('INSERT INTO speaking_items VALUES (?,?)', ('q1', json.dumps(catalog_item)))
                db.commit()
            preview = clean_bank(path)
            self.assertEqual(len(preview['changes']), 1)
            self.assertFalse(preview['applied'])
            self.assertEqual(path.read_text(encoding='utf-8'), raw)
            result = clean_bank(path, apply=True)
            expected = copy.deepcopy(bank)
            expected['items'][0]['topic'] = 'Sitting down'
            self.assertEqual(json.loads(path.read_text(encoding='utf-8')), expected)
            self.assertEqual((Path(result['backup']) / 'speaking.json').read_text(encoding='utf-8'), raw)
            with closing(sqlite3.connect(Path(folder) / 'catalog.db')) as db:
                saved = json.loads(db.execute('SELECT payload FROM speaking_items').fetchone()[0])
            catalog_item['topic'] = 'Sitting down'
            self.assertEqual(saved, catalog_item)
            self.assertFalse(clean_bank(path, apply=True)['applied'])
            self.assertFalse(path.with_suffix('.json.lock').exists())

    def test_existing_lock_prevents_changes(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / 'speaking.json'
            path.write_text('{}', encoding='utf-8')
            lock = path.with_suffix('.json.lock')
            lock.touch()
            with self.assertRaises(FileExistsError):
                clean_bank(path, apply=True)
            self.assertTrue(lock.exists())
            self.assertEqual(path.read_text(encoding='utf-8'), '{}')


if __name__ == '__main__':
    unittest.main()
