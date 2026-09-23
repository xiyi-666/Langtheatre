package graph

import (
	"context"
	"encoding/json"
	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
	"os"
	"strings"
	"testing"
	"time"
)

func TestImportedSpeakingGraph(t *testing.T) {
	path := os.Getenv("TEST_SPEAKING_BANK")
	if path == "" {
		t.Skip("licensed corpus is local only")
	}
	t.Setenv("SPEAKING_BANK_PATH", path)
	svc := service.New(store.NewMemoryStore(), nil, nil, nil, "test")
	schema, err := NewSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	query := `mutation { startSpeakingSession { id prompts { questionId source audioUrl part question cueCard } } }`
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, "owner")})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	raw, _ := json.Marshal(result.Data)
	if !strings.Contains(string(raw), ".pdf") || strings.Contains(string(raw), `"questionId":""`) {
		t.Fatal(string(raw))
	}
	result = graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.Background()})
	if len(result.Errors) == 0 {
		t.Fatal("unauthenticated bank session accepted")
	}
}

func TestLatestSpeakingSessionGraph(t *testing.T) {
	mem := store.NewMemoryStore()
	now := time.Now().UTC()
	for _, session := range []domain.SpeakingSession{
		{ID: "completed", UserID: "owner", Status: "COMPLETED", UpdatedAt: now.Add(time.Minute)},
		{ID: "active", UserID: "owner", Status: "ACTIVE", UpdatedAt: now},
		{ID: "foreign", UserID: "other", Status: "ACTIVE", UpdatedAt: now.Add(2 * time.Minute)},
	} {
		if _, err := mem.CreateSpeakingSession(session); err != nil {
			t.Fatal(err)
		}
	}
	svc := service.New(mem, nil, nil, nil, "test")
	schema, err := NewSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	query := `query { latestSpeakingSession { id status } }`
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, "owner")})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	raw, _ := json.Marshal(result.Data)
	if !strings.Contains(string(raw), `"id":"active"`) {
		t.Fatalf("unexpected latest session: %s", raw)
	}

	missing := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, "missing")})
	missingJSON, _ := json.Marshal(missing.Data)
	if len(missing.Errors) > 0 || !strings.Contains(string(missingJSON), `"latestSpeakingSession":null`) {
		t.Fatalf("missing session should be null: data=%s errors=%v", missingJSON, missing.Errors)
	}

	unauthorized := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.Background()})
	if len(unauthorized.Errors) == 0 {
		t.Fatal("unauthenticated latest session query accepted")
	}
}
