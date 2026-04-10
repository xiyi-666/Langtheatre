package graph

import (
	"encoding/json"
	"errors"

	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/service"
)

type ContextUserKey string

const UserIDKey ContextUserKey = "uid"

func NewSchema(svc *service.Service) (graphql.Schema, error) {
	dialogueType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Dialogue",
		Fields: graphql.Fields{
			"speaker":   &graphql.Field{Type: graphql.String},
			"text":      &graphql.Field{Type: graphql.String},
			"zhSubtitle": &graphql.Field{Type: graphql.String},
			"audioUrl":  &graphql.Field{Type: graphql.String},
			"timestamp": &graphql.Field{Type: graphql.Float},
		},
	})

	theaterQuizPublicType := graphql.NewObject(graphql.ObjectConfig{
		Name: "TheaterQuizQuestion",
		Fields: graphql.Fields{
			"question": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"options":  &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
		},
	})

	characterType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Character",
		Fields: graphql.Fields{
			"name":  &graphql.Field{Type: graphql.String},
			"role":  &graphql.Field{Type: graphql.String},
			"color": &graphql.Field{Type: graphql.String},
		},
	})

	theaterType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Theater",
		Fields: graphql.Fields{
			"id":               &graphql.Field{Type: graphql.String},
			"language":         &graphql.Field{Type: graphql.String},
			"topic":            &graphql.Field{Type: graphql.String},
			"difficulty":       &graphql.Field{Type: graphql.Float},
			"mode":             &graphql.Field{Type: graphql.String},
			"status":           &graphql.Field{Type: graphql.String},
			"isFavorite":       &graphql.Field{Type: graphql.Boolean},
			"shareCode":        &graphql.Field{Type: graphql.String},
			"sceneDescription": &graphql.Field{Type: graphql.String},
			"characters": &graphql.Field{
				Type: graphql.NewList(characterType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					theater, ok := p.Source.(domain.Theater)
					if !ok {
						return nil, errors.New("invalid theater source")
					}
					return theater.Characters, nil
				},
			},
			"dialogues": &graphql.Field{
				Type: graphql.NewList(dialogueType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					theater, ok := p.Source.(domain.Theater)
					if !ok {
						return nil, errors.New("invalid theater source")
					}
					return theater.Dialogues, nil
				},
			},
			"quizQuestions": &graphql.Field{
				Type: graphql.NewList(theaterQuizPublicType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					theater, ok := p.Source.(domain.Theater)
					if !ok {
						return nil, errors.New("invalid theater source")
					}
					public := make([]map[string]interface{}, 0, len(theater.QuizQuestions))
					for _, q := range theater.QuizQuestions {
						options := q.Options
						if options == nil {
							options = []string{}
						}
						public = append(public, map[string]interface{}{
							"question": q.Question,
							"options":  options,
						})
					}
					return public, nil
				},
			},
		},
	})

	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id":      &graphql.Field{Type: graphql.String},
			"email":   &graphql.Field{Type: graphql.String},
			"nickname": &graphql.Field{
				Type: graphql.String,
			},
			"avatarUrl": &graphql.Field{Type: graphql.String},
			"bio":       &graphql.Field{Type: graphql.String},
			"totalXP": &graphql.Field{Type: graphql.Int},
		},
	})

	authType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthPayload",
		Fields: graphql.Fields{
			"accessToken": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	practiceResultType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PracticeResult",
		Fields: graphql.Fields{
			"score":         &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"xpEarned":      &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"feedback":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"correctCount":  &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"totalCount":    &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
		},
	})

	courseType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Course",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.String},
			"language":    &graphql.Field{Type: graphql.String},
			"category":    &graphql.Field{Type: graphql.String},
			"title":       &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"minLevel":    &graphql.Field{Type: graphql.Float},
			"maxLevel":    &graphql.Field{Type: graphql.Float},
			"isActive":    &graphql.Field{Type: graphql.Boolean},
		},
	})

	roleplayType := graphql.NewObject(graphql.ObjectConfig{
		Name: "RoleplaySession",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.String},
			"userId":        &graphql.Field{Type: graphql.String},
			"theaterId":     &graphql.Field{Type: graphql.String},
			"userRole":      &graphql.Field{Type: graphql.String},
			"turnIndex":     &graphql.Field{Type: graphql.Int},
			"currentScore":  &graphql.Field{Type: graphql.Int},
			"status":        &graphql.Field{Type: graphql.String},
			"finalFeedback": &graphql.Field{Type: graphql.String},
			"transcript": &graphql.Field{
				Type: graphql.NewList(dialogueType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					session, ok := p.Source.(domain.RoleplaySession)
					if !ok {
						return nil, errors.New("invalid roleplay source")
					}
					return session.Transcript, nil
				},
			},
		},
	})

	generateInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "GenerateTheaterInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"language":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"topic":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"difficulty": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Float)},
			"mode":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"me": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					return svc.Me(userID)
				},
			},
			"theater": &graphql.Field{
				Type: theaterType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id := p.Args["id"].(string)
					return svc.Theater(id)
				},
			},
			"myTheaters": &graphql.Field{
				Type: graphql.NewList(theaterType),
				Args: graphql.FieldConfigArgument{
					"language": &graphql.ArgumentConfig{Type: graphql.String},
					"status":   &graphql.ArgumentConfig{Type: graphql.String},
					"favorite": &graphql.ArgumentConfig{Type: graphql.Boolean},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					language, _ := p.Args["language"].(string)
					status, _ := p.Args["status"].(string)
					var favorite *bool
					if v, ok := p.Args["favorite"].(bool); ok {
						favorite = &v
					}
					return svc.MyTheaters(userID, language, status, favorite)
				},
			},
			"courses": &graphql.Field{
				Type: graphql.NewList(courseType),
				Args: graphql.FieldConfigArgument{
					"language": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					language, _ := p.Args["language"].(string)
					return svc.ListCourses(language)
				},
			},
			"roleplaySession": &graphql.Field{
				Type: roleplayType,
				Args: graphql.FieldConfigArgument{
					"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					return svc.GetRoleplaySession(userID, p.Args["sessionId"].(string))
				},
			},
		},
	})

	mutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"register": &graphql.Field{
				Type: authType,
				Args: graphql.FieldConfigArgument{
					"email":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					token, err := svc.Register(p.Args["email"].(string), p.Args["password"].(string))
					if err != nil {
						return nil, err
					}
					return map[string]any{"accessToken": token}, nil
				},
			},
			"login": &graphql.Field{
				Type: authType,
				Args: graphql.FieldConfigArgument{
					"email":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					token, err := svc.Login(p.Args["email"].(string), p.Args["password"].(string))
					if err != nil {
						return nil, err
					}
					return map[string]any{"accessToken": token}, nil
				},
			},
			"refresh": &graphql.Field{
				Type: authType,
				Args: graphql.FieldConfigArgument{
					"accessToken": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					token, err := svc.Refresh(p.Args["accessToken"].(string))
					if err != nil {
						return nil, err
					}
					return map[string]any{"accessToken": token}, nil
				},
			},
			"logout": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return false, errors.New("unauthorized")
					}
					if err := svc.Logout(userID); err != nil {
						return false, err
					}
					return true, nil
				},
			},
			"updateProfile": &graphql.Field{
				Type: userType,
				Args: graphql.FieldConfigArgument{
					"nickname":  &graphql.ArgumentConfig{Type: graphql.String},
					"avatarUrl": &graphql.ArgumentConfig{Type: graphql.String},
					"bio":       &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					nickname, _ := p.Args["nickname"].(string)
					avatarURL, _ := p.Args["avatarUrl"].(string)
					bio, _ := p.Args["bio"].(string)
					return svc.UpdateProfile(userID, nickname, avatarURL, bio)
				},
			},
			"generateTheater": &graphql.Field{
				Type: theaterType,
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(generateInput)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					input := p.Args["input"].(map[string]any)
					raw, _ := json.Marshal(input)
					var payload struct {
						Language   string  `json:"language"`
						Topic      string  `json:"topic"`
						Difficulty float64 `json:"difficulty"`
						Mode       string  `json:"mode"`
					}
					_ = json.Unmarshal(raw, &payload)
					return svc.GenerateTheater(userID, payload.Language, payload.Topic, payload.Difficulty, payload.Mode)
				},
			},
			"submitAnswers": &graphql.Field{
				Type: practiceResultType,
				Args: graphql.FieldConfigArgument{
					"theaterId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"answers":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.String))},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					anyAnswers := p.Args["answers"].([]interface{})
					answers := make([]string, 0, len(anyAnswers))
					for _, item := range anyAnswers {
						answers = append(answers, item.(string))
					}
					return svc.SubmitAnswers(userID, p.Args["theaterId"].(string), answers)
				},
			},
			"toggleFavorite": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"theaterId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"favorite":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return false, errors.New("unauthorized")
					}
					if err := svc.ToggleFavorite(userID, p.Args["theaterId"].(string), p.Args["favorite"].(bool)); err != nil {
						return false, err
					}
					return true, nil
				},
			},
			"shareTheater": &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"theaterId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					return svc.ShareTheater(userID, p.Args["theaterId"].(string))
				},
			},
				"deleteTheater": &graphql.Field{
					Type: graphql.Boolean,
					Args: graphql.FieldConfigArgument{
						"theaterId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						userID, _ := p.Context.Value(UserIDKey).(string)
						if userID == "" {
							return false, errors.New("unauthorized")
						}
						if err := svc.DeleteTheater(userID, p.Args["theaterId"].(string)); err != nil {
							return false, err
						}
						return true, nil
					},
				},
			"startRoleplay": &graphql.Field{
				Type: roleplayType,
				Args: graphql.FieldConfigArgument{
					"theaterId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"userRole":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					return svc.StartRoleplay(userID, p.Args["theaterId"].(string), p.Args["userRole"].(string))
				},
			},
			"submitRoleplayReply": &graphql.Field{
				Type: roleplayType,
				Args: graphql.FieldConfigArgument{
					"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
					"text":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					return svc.SubmitRoleplayReply(userID, p.Args["sessionId"].(string), p.Args["text"].(string))
				},
			},
			"endRoleplay": &graphql.Field{
				Type: roleplayType,
				Args: graphql.FieldConfigArgument{
					"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					userID, _ := p.Context.Value(UserIDKey).(string)
					if userID == "" {
						return nil, errors.New("unauthorized")
					}
					return svc.EndRoleplay(userID, p.Args["sessionId"].(string))
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    query,
		Mutation: mutation,
	})
}
