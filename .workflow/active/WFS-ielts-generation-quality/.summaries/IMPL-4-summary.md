## Summary

Implemented Band-aware and question-type-aware reading generation prompts, with post-parse defaults and validators for IELTS-style question structures.

## Files Modified

- `apps/server/internal/ai/generator.go`
- `apps/server/internal/ai/generator_test.go`
- `apps/server/internal/service/service.go`

## Key Decisions

- Reading prompt now includes Band, Stage, question type, skill focus, scenario family, minimum word count, and strict task structure.
- Band controls minimum word count, so higher IELTS Bands require longer and denser passages.
- Post-generation validation rejects mismatched question types and missing structures for Matching Headings, Matching Information, TFNG, Summary Completion, and Multiple Choice.

## Tests

- `go test ./...` from `apps/server`: passed.
