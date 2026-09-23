import unittest
import tempfile
from pathlib import Path
from import_ielts_bank import parse
from link_ielts_audio import resolve_audio


class SpeakingImportTests(unittest.TestCase):
    def test_topic_watermark_is_removed_before_heading_is_saved(self):
        lines = [(1, '1. Sitting down abckg525 如果此素材不是从此微信购买，其他买家均为转卖，'),
                 (1, 'What is your favorite place to sit?')]
        items, rejected = parse(lines, 'fixture.pdf')
        self.assertFalse(rejected)
        self.assertEqual(items[0]['topic'], 'Sitting down')
        self.assertEqual(items[0]['question'], 'What is your favorite place to sit?')

    def test_audio_mapping_rejects_traversal(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            with self.assertRaises(ValueError):
                resolve_audio(root, '../private.mp3')

    def test_paired_questions_across_pages(self):
        lines = [(1, '1.Hobbies'), (1, 'What hobby do you enjoy?'),
                 (1, 'Part2&Part3'), (1, 'P2'), (1, 'Describe a skill you enjoy using'),
                 (1, 'You should say:'), (1, 'What it is'), (1, 'When you learned it'),
                 (1, 'Who helped you'), (2, '2 / 5'), (2, 'And explain why you enjoy using it'),
                 (2, 'P3'), (2, 'Why do people learn'), (2, 'new skills?'), (2, 'Why?')]
        items, rejected = parse(lines, 'fixture.pdf')
        self.assertEqual(len(items), 3)
        self.assertEqual(rejected, [])
        self.assertEqual(items[1]['group'], items[2]['group'])
        self.assertEqual(items[1]['sourcePage'], 1)
        self.assertEqual(items[2]['question'], 'Why do people learn new skills? Why?')

    def test_answer_is_not_a_question(self):
        items, _ = parse([(1, 'What do you enjoy studying?'), (1, 'I enjoy learning science every day.')], 'fixture.pdf')
        self.assertEqual(len(items), 1)

    def test_truncated_cue_is_rejected(self):
        items, rejected = parse([(1, 'Describe a friend'), (1, 'You should say:'), (1, 'Who they are')], 'fixture.pdf')
        self.assertEqual(items, [])
        self.assertEqual(len(rejected), 1)

    def test_reviewed_watermark_does_not_remove_cue_points(self):
        lines = [(4, text) for text in ['Describe something broken in your home and then repaired',
                 'You should say:', 'What it is 中文水印', 'How it was broken 中文水印',
                 'How you got it repaired', 'And how you felt about it']]
        items, rejected = parse(lines, '2022年5-8月雅思口语Part2题库.pdf')
        self.assertFalse(rejected)
        self.assertIn('What it is How it was broken', items[0]['cueCard'])


if __name__ == '__main__':
    unittest.main()
