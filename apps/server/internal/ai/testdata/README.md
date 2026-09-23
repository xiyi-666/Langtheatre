# Mock exam test fixtures

`mock_exam_valid.json` is deterministic SYNTHETIC TEST-ONLY data for AI gate and service lifecycle tests. Its approval flags simulate a successful mock-provider review. It is not educational content and must never be loaded by production code or used as a generation fallback.

From `apps/server`, export it with `IELTS_MOCK_EXPORT_FIXTURE=1 go test ./internal/ai -run '^TestMockExamExportFixture$' -count=1` (set the environment variable using your shell's syntax).

The fixture contains server-side answer keys. Do not expose it as a public client asset.
