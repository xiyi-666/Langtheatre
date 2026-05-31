## Summary

Improved TTS stability and reading audio repair. TTS now retries transient network errors plus 429/408/5xx responses with bounded backoff. Reading audio generation preserves successful chunks, resumes from partial audio, and exposes a retry mutation.

## Files Modified

- `apps/server/internal/ai/tts.go`
- `apps/server/internal/ai/tts_test.go`
- `apps/server/internal/service/service.go`
- `apps/server/internal/service/service_test.go`
- `apps/server/internal/graph/schema.go`
- `apps/client/src/api.ts`

## Key Decisions

- Kept provider behavior compatible while broadening retryable conditions.
- Partial reading audio failures now keep successful URLs and leave status `PENDING` for retry.
- Added `RetryReadingAudio` service method and GraphQL/client API wrapper.

## Tests

- `go test ./...` from `apps/server`: passed.
- `npm run typecheck --workspace apps/client`: passed.
- `npm run lint --workspace apps/client`: passed.
