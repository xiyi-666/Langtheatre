## Summary

Added a small-sample quality gate for pre-expansion validation. The gate checks 5 listening samples and 5 reading samples across Bands, Sections, and reading question types.

## Files Modified

- `apps/server/internal/service/quality_gate_test.go`
- `docs/test-report.md`

## Key Decisions

- Implemented the quality gate as a targeted Go test so it can run in CI or locally without external AI/TTS credentials.
- Gate checks objective quality properties: prompt leakage, English spacing, paragraph/word count, question structure, metadata coverage, and fallback readiness.
- Test report states that real AI-generated samples still require manual review before expanding to 300 items.

## Tests

- `go test ./internal/service -run TestSmallSampleQualityGate -v` from `apps/server`: passed.
- `go test ./...` from `apps/server`: passed.
