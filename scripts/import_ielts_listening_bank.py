"""Scan local IELTS listening material into a reviewable candidate manifest.

This importer deliberately does not publish content. A listening paper is only
eligible for publication after its answer key and OCR have been reviewed.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def pdf_text(path: Path) -> str:
    import pdfplumber

    with pdfplumber.open(path) as document:
        return "\n".join(page.extract_text() or "" for page in document.pages)


def audio_duration(path: Path) -> float | None:
    ffprobe = shutil.which("ffprobe")
    if not ffprobe:
        return None
    result = subprocess.run(
        [ffprobe, "-v", "error", "-show_entries", "format=duration",
         "-of", "default=noprint_wrappers=1:nokey=1", str(path)],
        capture_output=True, text=True, check=False,
    )
    try:
        return round(float(result.stdout.strip()), 2)
    except (TypeError, ValueError):
        return None


def find_source(root: Path) -> Path:
    matches = [p for p in root.rglob("CambridgeIeltsV4 01.pdf")]
    if not matches:
        raise FileNotFoundError("未找到 CambridgeIeltsV4 01.pdf")
    return matches[0].parent


def build_candidate(root: Path, source_dir: Path, test: int) -> dict:
    pdf = source_dir / f"CambridgeIeltsV4 0{test}.pdf"
    # Only accept section audio from the same book directory. Never borrow an
    # audio folder from another Cambridge volume based on a matching test number.
    audio_dir = next(
        (p for p in source_dir.parent.rglob(f"test{test}") if p.is_dir()),
        source_dir / f"test{test}",
    )
    text = pdf_text(pdf) if pdf.exists() else ""
    sections = []
    for part in range(1, 5):
        marker = re.search(rf"SECTION\s+{part}\b", text, re.I)
        next_marker = re.search(rf"SECTION\s+{part + 1}\b", text[marker.end():], re.I) if marker else None
        end = marker.end() + next_marker.start() if marker and next_marker else len(text)
        section_text = text[marker.end():end].strip() if marker else ""
        audio = audio_dir / f"section{part}.mp3"
        sections.append({
            "id": f"cambridge-4-test-{test}-section-{part}",
            "part": part,
            "questionCount": len(re.findall(r"\b(?:Questions?|Question)\s+\d+", section_text, re.I)),
            "questionText": section_text,
            "audioPath": str(audio) if audio.exists() else "",
            "audioSha256": sha256(audio) if audio.exists() else "",
            "audioDurationSeconds": audio_duration(audio) if audio.exists() else None,
            "answerKeyStatus": "MISSING",
            "reviewStatus": "REVIEW_REQUIRED",
        })
    return {
        "id": f"cambridge-4-test-{test}",
        "source": "F:/BaiduNetdiskDownload/雅思/06.雅思听力",
        "book": "Cambridge IELTS 4",
        "test": test,
        "sourcePdf": str(pdf),
        "sourcePdfSha256": sha256(pdf) if pdf.exists() else "",
        "rights": "USER_PROVIDED_LICENSED_SOURCE",
        "status": "REVIEW_REQUIRED",
        "blockedReasons": ["PDF OCR needs review", "official answer key not found in source bundle"],
        "sections": sections,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    source_dir = find_source(args.source)
    papers = [build_candidate(args.source, source_dir, test) for test in range(1, 5)]
    payload = {
        "schemaVersion": 1,
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "publishPolicy": "Only records with verified answerKey, OCR review, audio validation and difficulty review may be published.",
        "papers": papers,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"papers": len(papers), "sections": len(papers) * 4, "output": str(args.output)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
