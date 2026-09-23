# IELTS Academic 6.0–8.0 validation

Status: source-first implemented; one real full paper at each target 6/7/8 passed generation, blind review and latest-code offline replay. Complete target-7 audio decode also passed. No claim of official equivalence or a 100% first-attempt success rate.

1. Replace placeholder paper with AI-generated original full-length sections and independent answer review; reject provider/quality failure.
2. Require 3 reading passages (2,150–2,750 words total), 4 listening parts with complete scripts, 40 questions each, diverse supported question types and source evidence. Writing Task 1 must include complete data.
3. Use IELTS writing criteria, Task 1:Task 2 weight 1:2, strict score validation, approximate raw-score reference bands, and no overall IELTS score without speaking.
4. Queue generation and audio; begin timing only after readiness. Persist answers, enforce ownership, prevent duplicate costly operations, expose actionable Chinese errors.
5. Test malformed/ambiguous/unsupported content, provider failure, scoring boundaries and API lifecycle. Run real configured provider checks and retain non-secret results.
6. Target 6.0, 7.0 and 8.0 are learner goals, not certified item difficulty. Automated checks and model review do not replace examiner-reviewed samples or candidate calibration.

Progress

- [x] Baseline gaps and test coverage inspected.
- [x] Content generation and semantic review implementation (live quality acceptance remains below).
- [x] Scoring and service/frontend integration.
- [x] Unit/integration/build validation.
- [x] Real writing provider validation (Task 2 contrast/injection matrix and Task 1 data report).
- [x] Real listening TTS delivery/decoding/duration sanity check (one draft, not full-paper approval).
- [x] Full paper real-provider acceptance at targets 6.0 / 7.0 / 8.0 (one approved paper each, including recorded failed runs and retests).

Reference sources

- https://ielts.org/take-a-test/your-results/ielts-scoring-in-detail (checked 2026-09-14): Listening reference points 16/23/30/35 for Bands 5/6/7/8; Academic Reading 15/23/30/35. Exact conversion varies by form.
- https://ielts.org/take-a-test/test-types/ielts-academic-test/ielts-academic-format-reading (checked 2026-09-14): 3 sections, 2,150–2,750 words, 40 questions, 60 minutes; ordered multiple-choice/completion groups and paragraph matching are legitimate task conventions.
- https://ielts.org/take-a-test/test-types/ielts-academic-test/ielts-academic-format-listening (checked 2026-09-14): 4 parts with 10 questions each, chronological questions, everyday settings in Parts 1/2 and approximately 30 minutes.
- This implementation is three-skill training: a 150-minute overall deadline, suggested subject timings, replayable audio, no Speaking. It must not be described as an official full IELTS exam.

Reproducible checks (run backend commands in `apps/server`)

```text
go test ./... -count=1 -timeout=120s
go vet ./...
IELTS_MOCK_LIVE=1 go test ./internal/ai -run '^TestLiveMockExam$' -count=1 -timeout=65m -v
IELTS_WRITING_LIVE=1 go test ./internal/ai -run '^TestLiveWritingEvaluation$' -count=1 -timeout=15m -v
```

The last two commands use configured paid providers and must be explicitly opted into; automated CI skips them. Unit tests use synthetic fixtures strictly under `internal/ai/testdata`, never production fallback material. In PowerShell set `$env:IELTS_MOCK_LIVE='1'` / `$env:IELTS_WRITING_LIVE='1'` before the corresponding command. Credentials must remain in local configuration, not logs or reports.

Verified local results (2026-09-14)

- Backend: `go test ./... -count=1 -timeout=120s` passed; `go vet ./...` passed.
- Frontend: 25 unit tests passed; lint has zero errors and three existing Fast Refresh warnings. Production build passed with the existing large-bundle warning.
- Frontend stateful regressions: `npm run test:mock-exam --workspace apps/client` passed two flows with 33 named checks. Uses mocked GraphQL and silent audio, not real providers. Covers generation cost, readiness, draft persistence, expiry, evaluation retry, ordered playback, writing criteria/evidence and desktop/mobile layout. CI runs this once in the mini-program matrix job. Playwright is explicitly declared in the client package.
- Writing reports now persist both complete task evaluations, including criterion scores and evidence. Old reports without these fields do not acquire fabricated details.

Real writing evaluation

The provider was `OPENAI_COMPATIBLE / gpt-5.6-terra`, resolved from the local SQLite persisted configuration rather than merely from `.env`. No credentials are included here.

| Self-authored sample | Raw criterion mean | Display Band | Outcome |
| --- | ---: | ---: | --- |
| Task 2 weak | 3.000 | 3.0 | Four valid evidence quotations |
| Task 2 medium | 6.875 | 7.0 | Four valid evidence quotations |
| Task 2 strong | 7.875 | 8.0 | Four valid quotations after one correction retry |
| Task 2 injected weak | 3.000 | 3.0 | Did not follow embedded request for Band 9 |
| Task 1 numeric-table description | 8.000 | 8.0 | Four valid quotations; taskAchievement selected |

Task 2 matrix elapsed 205 seconds; Task 1 elapsed 36.5 seconds. These are self-authored comparison samples, NOT examiner-certified score anchors. A successful ordering and injection check does not establish calibration, repeatability, or official equivalence. Provider errors fail immediately; malformed or invalid assessment output gets at most one correction request, still subject to full validation. No generated substitute scores are used.

Task 1 reproduction: `IELTS_WRITING_LIVE=1 go test ./internal/ai -run '^TestLiveWritingTask1Evaluation$' -count=1 -timeout=4m -v`.

Real listening audio

- Provider/model: Xiaomi / `mimo-v2.5-tts`, using persisted local TTS configuration.
- One original listening draft: 24 turns, two speakers, 760 spoken words.
- All 24 returned segments were locally materialized and ffprobe-decodable: 321.6 seconds total, 141.8 words/minute; generation took 100.3 seconds.
- Offline signal scan of the concatenated sample: mean volume -18.1 dB; no silence interval of at least two seconds at the -45 dB threshold was reported.
- Private local evidence: `.workflow/ielts-live/audio-check/audio-report.json`; re-encoded listening preview: `.workflow/ielts-live/audio-check/ielts-listening-preview.mp3`. Not published or inserted into user exam records.
- This does not certify pronunciation, accent, speaker consistency, transcript alignment or human listening quality; those remain separate listening-review tasks. The source was a generated draft, not a released full paper.
- Opt-in test: `TestLiveMockListeningAudio`, using `IELTS_AUDIO_LIVE=1`, `IELTS_AUDIO_DRAFT`, absolute `IELTS_AUDIO_OUTPUT_DIR`, and `IELTS_AUDIO_FFPROBE`. The harness reads SQLite configuration without migrations or writes. Default CI skips paid calls.

Known full-paper live findings under investigation

- Initial target 6.0 generation was rejected after three attempts: Listening Part 2 matching option/source context mismatch. No paper released.
- Initial writing matrix failed validation; the corrected full matrix subsequently passed as recorded above. No replacement scores were supplied on failed attempts.
- First complete target matrix on persisted gpt-5.6-terra rejected all three papers: target 6 listening source length (900 vs 650–850 words), target 7 completion format, target 8 completion answer/evidence mismatch. These failures remain evidence, not passes.
- After format/prompt fixes, a further target-7 full-paper test advanced beyond the initial sections but failed after 303.9 seconds: Listening Part 3 still contained 929 words (allowed 650–850) after three attempts. Evidence: `.workflow/ielts-live/ielts-academic-v2-1896330704/target-7-failure.json`.
- At that earlier checkpoint no successful full-paper report was available. Early listening failures cancelled remaining jobs, so those runs did not establish acceptance of reading. See resumed evidence below for later successful real full-paper validation; unit fixtures are not substitutes.
- Next work: improve source length control without dropping answer-bearing evidence or lowering quality checks; run targeted reading/section validation to avoid repeatedly paying for already-checked sections; then rerun the whole 6/7/8 matrix. Examiner-reviewed anchor responses and candidate calibration remain required before any official-equivalence claim.

Local handoff

- Frontend: `http://localhost:5174/mock-exam`; backend: port 8177, healthy with Redis up. Local SQLite is used, so health reports PostgreSQL as not configured.
- Latest verified backend executable was started after the saved source edits. Browser history query succeeded after refresh, and the mini-program cost query displays no credit deduction.
- No Git commit, push, production deployment, or publication of private generated materials was performed.

Resumed development: source-first generation

- [x] Separate source drafting from question authoring. Runtime counts the complete source before questions can be generated. Source-only correction is bounded to initial + two attempts; no programmatic truncation or inserted filler.
- [x] Freeze accepted source for all question repairs and blind review. Reject question-stage outputs that attempt to change it. Source, question and review calls share the existing two-request limit.
- [x] Regression tests cover bounded source correction, no questions/review on invalid source, immutable source on question repair, and private capture without credentials.
- [x] Independently validate Listening 3, Reading 1/2/3 and Writing using the configured real provider (separate batches, not one released paper).
- [x] Repeat full-paper target 6/7/8 acceptance and update runtime after final checks.

Resumed live evidence (2026-09-14, local time)

- Independent batch `ielts-sections-1543460620`: Listening 3 passed (769 words, 10 questions); Reading 2 passed (801 words, 13 questions); two Writing tasks passed. Reading 1/3 were rejected for weak distractors/difficulty and an ambiguous completion item. These failures motivated question-authoring changes, not reduced quality gates.
- Retest `ielts-sections-2157886471`: Reading 1 passed (786 words, 13 questions, 187 seconds); Reading 3 passed (823 words, 14 questions, 158 seconds). All returned keys agreed with blind answers. These automated reviews are not examiner calibration; occasional awkward phrasing and weaker distractors remain a manual editorial concern.
- Earlier full target-6 run `ielts-academic-v2-603089895` passed all four Listening parts and Reading 1, but the old 10-minute harness deadline cancelled Reading 2/3. No complete paper was returned. Full-paper live timeout now matches the 20-minute production task budget; audio still shares the production job budget.
- Quote-only review correction: at most one extra blind review of unchanged questions. All semantic flags, uniqueness/support, answer agreement and feedback must already pass before this route is allowed. The correction receives no author answers/evidence. Continued invalid quotes or provider errors still fail; no replacement evidence is used to approve a paper.
- Matrix `ielts-academic-v2-3539555916`: target 6 failed Listening 4 question quality after 3 attempts; target 7 passed all 8 sections in 624 seconds; target 8 failed after 344 seconds on a 4-word completion with a 3-word limit, using the old shared three-attempt budget. Target-7 listening words: 761/772/742/748; reading words: 825/810/830 (2,465 total), questions 40+40, writing tasks 2. Latest-code offline replay also verified every persisted answer against its final blind review. The matrix command exited nonzero; later individual passing retests must not erase those failures.
- Latest target-6 retest PASSED in `ielts-academic-v2-3014544031` (425 seconds), with numeric evidence protection, improved listening authoring and separately bounded format/quality repairs. Listening words 790/743/690/755; reading words 821/803/784 (2,408 total); 40+40 questions and 2 writing tasks. Latest-code offline replay verified all eight final reviews and every persisted answer.
- Latest target-8 retest PASSED in `ielts-academic-v2-3519697475` (558 seconds). Listening words 761/763/783/744; reading words 869/811/877 (2,557 total); 40+40 questions and 2 writing tasks. Reading 3's discontinuous quotation was rejected twice and corrected before blind review. No programmatic substitute source/answers were inserted.
- Whole-paper TTS harness accepts `IELTS_AUDIO_PAPER` instead of `IELTS_AUDIO_DRAFT`; validates the complete approved paper before paid TTS, checks all four parts and every segment. A draft and a paper may not be supplied together. Uses local private output and read-only SQLite configuration, not user exam records.
- Numeric evidence now preserves signs, percentages, currency and numeric separators, including at quotation boundaries. Regression rejects +15 versus -15 and 1.000 versus 1,000; no semantic repair disguised as punctuation normalization.
- Audio validation now forces an audio stream and complete FFmpeg decode before duration/rate checks. Offline regression passes for a valid AAC signal and rejects a corrupted AAC payload and a video-only file. Earlier metadata-only results are not retroactively treated as full-decode evidence.
- Local formatting failures and semantic rejections have separate budgets: at most 3 failures of either class, at most 5 question-generation requests per section, and at most 1 quote-only review correction per reviewed draft. This does not weaken any quality check.

Full reviewed-paper audio acceptance (2026-09-14)

- Input: approved target-7 private full paper, not an unapproved draft. Provider Xiaomi `mimo-v2.5-tts`; 63 segments across all 4 listening parts. Each segment has an audio stream, completes strict FFmpeg decoding, and passes duration/rate sanity checks.
- Total 3,023 words, 1,276.48 seconds (21 minutes 16.5 seconds), 142.1 words/minute; TTS elapsed 357.4 seconds. Per-part seconds/rate: 329.6/138.5, 284.5/162.8, 327.8/135.8, 334.6/134.1.
- All four merged previews also decoded successfully; signal scan at -45 dB found no silence event lasting at least 2 seconds. This is a signal check, not human listening approval.
- Private report: `.workflow/ielts-live/audio-full-paper-7/audio-report.json`. Four locally merged listening previews: `listening-part-1.mp3` through `listening-part-4.mp3` in the same directory.
- This validates actual delivery, audio decoding and broad pacing, not human pronunciation/accent review or transcript alignment certification. `humanListeningReviewed` remains false. No user exam records were created and no scores were fabricated.
- Local runtime updated to `.workflow/ielts-runtime/server-source-first.exe`; port 8177 `/healthz` and GraphQL work, Redis up, frontend port 5174 GraphQL proxy works. No production deployment or Git push.

Acceptance summary

| Learner target | Reading words | Listening / Reading questions | Writing tasks | Real generation + blind review |
| --- | ---: | --- | ---: | --- |
| 6.0 | 2,408 | 40 / 40 | 2 | Passed after implementation changes, 425 s |
| 7.0 | 2,465 | 40 / 40 | 2 | Passed in initial resumed matrix, 624 s |
| 8.0 | 2,557 | 40 / 40 | 2 | Passed in latest retest, 558 s |

The complete matrix first returned failure (6/8 rejected); individual retests after targeted fixes passed. These are small-sample engineering/content-gate results, not statistical reliability evidence or calibrated IELTS band difficulty. Independent examiner review, officially scored anchor responses, candidate piloting and human audio/transcript review remain prerequisites for an equivalence claim. Existing provider/quality failures continue to produce system errors, never fabricated papers or scores.

Independent-section command (PowerShell, `apps/server`):

```powershell
$env:IELTS_MOCK_SECTIONS_LIVE='1'
$env:IELTS_MOCK_SECTIONS_OUTPUT_DIR='D:/project/pythonPJ/AIPlatorm/.workflow/ielts-live'
$env:IELTS_MOCK_SECTIONS='LISTENING_3,READING_1,READING_2,READING_3,WRITING'
$env:IELTS_MOCK_SECTIONS_BAND='7'
go test ./internal/ai -run '^TestLiveMockExamSections$' -v -count=1 -timeout=55m
```
