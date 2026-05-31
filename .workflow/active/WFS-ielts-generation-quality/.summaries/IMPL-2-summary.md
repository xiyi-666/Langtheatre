## Summary

Implemented IELTS listening Band and Section profiles. Listening generation now parses Band/Section/Focus/Task design hints from topic text, uses profile-derived question counts, and injects Section-specific task constraints into prompts.

## Files Modified

- `apps/server/internal/ielts/profile.go`
- `apps/server/internal/ielts/profile_test.go`
- `apps/server/internal/ai/generator.go`
- `apps/server/internal/service/service.go`

## Key Decisions

- Centralized IELTS tag/profile parsing in `internal/ielts`.
- Kept existing topic-based generation compatible while making Band and Section explicit in backend prompt construction.
- Preserved the existing 2/3 quiz-count behavior while making difficulty affect language, paraphrase, distractors, and task design.

## Tests

- `go test ./...` from `apps/server`: passed.
