package graph

import (
	"testing"

	"github.com/graphql-go/graphql"
)

func TestTheaterQuizQuestionContainsAnswerKeyField(t *testing.T) {
	schema, err := NewSchema(nil)
	if err != nil {
		t.Fatalf("new schema failed: %v", err)
	}

	result := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: `query {
      __type(name: "TheaterQuizQuestion") {
        fields { name }
      }
    }`,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("introspection query failed: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected response type: %T", result.Data)
	}
	typeData, ok := data["__type"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing __type data: %#v", data)
	}
	fields, ok := typeData["fields"].([]interface{})
	if !ok {
		t.Fatalf("missing fields data: %#v", typeData)
	}

	for _, f := range fields {
		field, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if field["name"] == "answerKey" {
			return
		}
	}
	t.Fatalf("field answerKey not found in TheaterQuizQuestion")
}
