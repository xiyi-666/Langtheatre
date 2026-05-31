## Summary

Upgraded fallback generation for listening and reading. Reading fallback now emits Band/question-type-aware multi-paragraph content with real Matching Headings, Matching Information, TFNG, Summary Completion, Mixed, and Multiple Choice structures. Listening fallback now follows Section-specific IELTS task patterns.

## Files Modified

- `apps/server/internal/service/service.go`
- `apps/server/internal/service/service_test.go`

## Key Decisions

- AI reading failures now route to structured fallback instead of failing the whole material generation path.
- The older generic reading fallback branch now delegates to the same metadata-aware fallback builder.
- Listening fallback uses the same `internal/ielts` Section profile as AI prompts.

## Tests

- `go test ./...` from `apps/server`: passed.
