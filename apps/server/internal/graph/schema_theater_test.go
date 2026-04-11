package graph

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
)

func TestTheaterTypeIncludesQuizQuestions(t *testing.T) {
	svc := service.New(store.NewMemoryStore(), nil, nil, nil, "test-secret")
	schema, err := NewSchema(svc)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	q := `{ __type(name: "Theater") { fields { name } } }`
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}
	data, _ := json.Marshal(result.Data)
	if !bytes.Contains(data, []byte(`"quizQuestions"`)) {
		t.Fatalf("Theater type missing quizQuestions in introspection: %s", data)
	}
}
