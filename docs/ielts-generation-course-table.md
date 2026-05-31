# IELTS Listening and Reading Generation Course Table

## Purpose

This table is the source curriculum for IELTS listening and reading generation. It is designed for small-sample QA first, then balanced batch expansion.

- Listening rows should be sent to `generateTheater` with `language=ENGLISH`, `mode=LISTENING`, `difficulty=band`, and `topic=generation_topic`.
- Reading rows should be sent to `generateReading` with `exam=IELTS`, `level=level`, and `topic=generation_topic`.
- The CSV version is the machine-readable source: `docs/ielts-generation-course-table.csv`.

## Generation Label Contract

Use the labels inside `generation_topic` because the backend parser reads them directly:

- `[Stage NN]` records the learning stage.
- `[Band X.X]` controls difficulty.
- `[Section N]` controls listening section behavior and reading passage metadata.
- `[Focus: ...]` controls the skill emphasis.
- `[Task design: ...]` controls listening task shape.
- Reading question type labels must use one of: `Multiple Choice`, `Matching Information`, `Matching Headings`, `TFNG`, `Summary Completion`, `Mixed Question Set`.

## Batch Balance

The course table has 24 generation modules: 12 listening and 12 reading. `target_count` is set so a full expansion produces 300 items:

- Listening: 150 items
- Reading: 150 items
- Stage 01-06: 12 items per skill module
- Stage 07-12: 13 items per skill module

Before generating the full batch, run a QA sample with 5 listening rows and 5 reading rows across different Bands, Sections, and question types.

## Stage Overview

| Stage | Band | Listening Target | Reading Target |
|---|---:|---|---|
| Stage 01 | 5.0 | Section 1 service details, names, numbers, spelling | Passage 1 concrete topic, Multiple Choice |
| Stage 02 | 5.0 | Section 1 transactional booking and corrections | Passage 1 practical topic, Matching Information |
| Stage 03 | 5.5 | Section 2 public orientation and route details | Passage 1/2 concrete claims, TFNG |
| Stage 04 | 5.5 | Section 2 facility description and sequence | Passage 2 paragraph main ideas, Matching Headings |
| Stage 05 | 6.0 | Section 3 academic planning and decisions | Passage 2 process text, Summary Completion |
| Stage 06 | 6.0 | Section 3 evidence comparison and mild disagreement | Passage 2 evidence location, Multiple Choice |
| Stage 07 | 6.5 | Section 4 lecture with cause-effect links | Passage 2/3 claim verification, TFNG |
| Stage 08 | 6.5 | Section 4 note completion with signposting | Passage 3 research/process summary, Summary Completion |
| Stage 09 | 7.0 | Section 1 dense transactional details with delayed answers | Passage 3 multi-skill set, Mixed Question Set |
| Stage 10 | 7.0 | Section 3 seminar opinions and decision reasons | Passage 3 abstract paragraph structure, Matching Headings |
| Stage 11 | 7.5 | Section 4 abstract lecture and competing interpretations | Passage 3 dense academic argument, Mixed Question Set |
| Stage 12 | 8.0 | Section 4 full-test style monologue with high paraphrase | Passage 3 high-density review set, Mixed Question Set |

## Quality Gates

Each generated item should satisfy these checks before saving to a large batch:

- No prompt leakage such as `Task design`, `Create an IELTS`, or instruction text in final passage/dialogue.
- English whitespace is readable; no fused text like `Goodafternoon,BrookdaleLanguageCentre`.
- Reading passages meet the Band-based paragraph and word-count rules.
- Reading questions match the declared question type.
- Listening questions match the declared Section type.
- `band`, `stage`, `section`, `skill_focus`, `question_type`, and `scenario_family` are visible in metadata where applicable.
