## Summary

Added additive reading metadata and structured reading question fields across domain, persistence, GraphQL, and client API/types.

## Files Modified

- `apps/server/internal/domain/models.go`
- `apps/server/internal/graph/schema.go`
- `apps/server/internal/store/sqlite_store.go`
- `apps/server/internal/store/postgres_store.go`
- `apps/server/migrations/001_init.sql`
- `apps/server/migrations/004_reading_metadata.sql`
- `apps/client/src/api.ts`
- `apps/client/src/types.ts`
- `apps/server/internal/service/service.go`
- `apps/server/internal/ai/generator.go`

## Key Decisions

- Added metadata fields as backward-compatible additions: `band`, `stage`, `section`, `skillFocus`, `questionType`, and `scenarioFamily`.
- Extended `QuizQuestion` with optional fields for matching, TFNG, summary completion, and evidence while preserving `question/options/answerKey`.
- Added service-side metadata inference for old rows with empty metadata.

## Tests

- `go test ./...` from `apps/server`: passed.
- `npm run typecheck --workspace apps/client`: passed after `npm ci`.
- `npm run lint --workspace apps/client`: passed.
