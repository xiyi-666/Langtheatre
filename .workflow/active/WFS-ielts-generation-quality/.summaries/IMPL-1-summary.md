## Summary

Implemented shared generation quality guards for prompt leakage, collapsed English spacing, word/paragraph counts, and generic reading questions. The AI generator and service layer now normalize English output before persistence/TTS and reject hard quality failures before saving reading material.

## Files Modified

- `apps/server/internal/contentquality/quality.go`
- `apps/server/internal/contentquality/quality_test.go`
- `apps/server/internal/ai/generator.go`
- `apps/server/internal/service/service.go`

## Key Decisions

- Added `internal/contentquality` so AI and service layers share the same checks without introducing an import cycle.
- Normalized repairable English spacing before validation, so examples like `Goodafternoon,BrookdaleLanguageCentre` become readable.
- Treated prompt leaks and repeated generic reading questions as hard generation failures.

## Tests

- `go test ./...` from `apps/server`: passed.
