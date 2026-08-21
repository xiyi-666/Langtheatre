package graph

import (
	"encoding/json"
	"errors"
	"time"

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
			"speaker":    &graphql.Field{Type: graphql.String},
			"text":       &graphql.Field{Type: graphql.String},
			"zhSubtitle": &graphql.Field{Type: graphql.String},
			"audioUrl":   &graphql.Field{Type: graphql.String},
			"timestamp":  &graphql.Field{Type: graphql.Float},
		},
	})

	readingStatementType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ReadingStatement",
		Fields: graphql.Fields{
			"id":     &graphql.Field{Type: graphql.String},
			"text":   &graphql.Field{Type: graphql.String},
			"answer": &graphql.Field{Type: graphql.String},
		},
	})

	theaterQuizPublicType := graphql.NewObject(graphql.ObjectConfig{
		Name: "TheaterQuizQuestion",
		Fields: graphql.Fields{
			"question":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"options":      &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			"answerKey":    &graphql.Field{Type: graphql.String},
			"type":         &graphql.Field{Type: graphql.String},
			"paragraphRef": &graphql.Field{Type: graphql.String},
			"evidence":     &graphql.Field{Type: graphql.String},
			"headings":     &graphql.Field{Type: graphql.NewList(graphql.String)},
			"statements":   &graphql.Field{Type: graphql.NewList(readingStatementType)},
			"summaryText":  &graphql.Field{Type: graphql.String},
			"wordBank":     &graphql.Field{Type: graphql.NewList(graphql.String)},
			"answers":      &graphql.Field{Type: graphql.NewList(graphql.String)},
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
			"id":                 &graphql.Field{Type: graphql.String},
			"language":           &graphql.Field{Type: graphql.String},
			"topic":              &graphql.Field{Type: graphql.String},
			"difficulty":         &graphql.Field{Type: graphql.Float},
			"mode":               &graphql.Field{Type: graphql.String},
			"status":             &graphql.Field{Type: graphql.String},
			"generationProgress": &graphql.Field{Type: graphql.Int},
			"generationMessage":  &graphql.Field{Type: graphql.String},
			"isFavorite":         &graphql.Field{Type: graphql.Boolean},
			"shareCode":          &graphql.Field{Type: graphql.String},
			"sceneDescription":   &graphql.Field{Type: graphql.String},
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
							"question":     q.Question,
							"options":      options,
							"type":         q.Type,
							"paragraphRef": q.ParagraphRef,
							"evidence":     q.Evidence,
							"headings":     q.Headings,
							"statements":   q.Statements,
							"summaryText":  q.SummaryText,
							"wordBank":     q.WordBank,
							"answers":      q.Answers,
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
			"id":            &graphql.Field{Type: graphql.String},
			"username":      &graphql.Field{Type: graphql.String},
			"email":         &graphql.Field{Type: graphql.String},
			"emailVerified": &graphql.Field{Type: graphql.Boolean},
			"nickname": &graphql.Field{
				Type: graphql.String,
			},
			"avatarUrl":     &graphql.Field{Type: graphql.String},
			"bio":           &graphql.Field{Type: graphql.String},
			"totalXP":       &graphql.Field{Type: graphql.Int},
			"level":         &graphql.Field{Type: graphql.Int},
			"xpIntoLevel":   &graphql.Field{Type: graphql.Int},
			"xpToNextLevel": &graphql.Field{Type: graphql.Int},
			"levelProgress": &graphql.Field{Type: graphql.Int},
			"rankCode":      &graphql.Field{Type: graphql.String},
			"rankLabel":     &graphql.Field{Type: graphql.String},
		},
	})

	billingProductType := graphql.NewObject(graphql.ObjectConfig{Name: "BillingProduct", Fields: graphql.Fields{
		"code": &graphql.Field{Type: graphql.String}, "name": &graphql.Field{Type: graphql.String}, "kind": &graphql.Field{Type: graphql.String}, "amountCents": &graphql.Field{Type: graphql.Int}, "creditAllowance": &graphql.Field{Type: graphql.Int}, "periodDays": &graphql.Field{Type: graphql.Int}, "adsFree": &graphql.Field{Type: graphql.Boolean}, "description": &graphql.Field{Type: graphql.String},
	}})
	billingStatusType := graphql.NewObject(graphql.ObjectConfig{Name: "BillingStatus", Fields: graphql.Fields{
		"productCode": &graphql.Field{Type: graphql.String}, "productName": &graphql.Field{Type: graphql.String}, "isLifetime": &graphql.Field{Type: graphql.Boolean}, "adsFree": &graphql.Field{Type: graphql.Boolean}, "creditBalance": &graphql.Field{Type: graphql.Int}, "creditAllowance": &graphql.Field{Type: graphql.Int},
		"creditResetAt": &graphql.Field{Type: graphql.String, Resolve: billingStatusTimeResolver(func(item domain.BillingStatus) time.Time { return item.CreditResetAt })},
		"expiresAt":     &graphql.Field{Type: graphql.String, Resolve: billingStatusTimeResolver(func(item domain.BillingStatus) time.Time { return item.ExpiresAt })},
	}})
	aiCreditCostType := graphql.NewObject(graphql.ObjectConfig{Name: "AICreditCost", Fields: graphql.Fields{
		"action": &graphql.Field{Type: graphql.String}, "label": &graphql.Field{Type: graphql.String}, "credits": &graphql.Field{Type: graphql.Int}, "description": &graphql.Field{Type: graphql.String},
	}})
	paymentOrderType := graphql.NewObject(graphql.ObjectConfig{Name: "PaymentOrder", Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.String}, "productCode": &graphql.Field{Type: graphql.String}, "amountCents": &graphql.Field{Type: graphql.Int}, "paymentChannel": &graphql.Field{Type: graphql.String}, "status": &graphql.Field{Type: graphql.String}, "checkoutURL": &graphql.Field{Type: graphql.String},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: paymentOrderTimeResolver(func(item domain.PaymentOrder) time.Time { return item.CreatedAt })},
		"paidAt":    &graphql.Field{Type: graphql.String, Resolve: paymentOrderTimeResolver(func(item domain.PaymentOrder) time.Time { return item.PaidAt })},
	}})
	xpEventType := graphql.NewObject(graphql.ObjectConfig{Name: "XPEvent", Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.String}, "activity": &graphql.Field{Type: graphql.String}, "sourceId": &graphql.Field{Type: graphql.String}, "xpEarned": &graphql.Field{Type: graphql.Int},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: xpEventTimeResolver},
	}})
	adPlacementType := graphql.NewObject(graphql.ObjectConfig{Name: "AdPlacement", Fields: graphql.Fields{
		"placement": &graphql.Field{Type: graphql.String}, "provider": &graphql.Field{Type: graphql.String}, "scriptURL": &graphql.Field{Type: graphql.String}, "slotId": &graphql.Field{Type: graphql.String},
	}})

	modelConfigType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ModelConfig",
		Fields: graphql.Fields{
			"provider":      &graphql.Field{Type: graphql.String},
			"model":         &graphql.Field{Type: graphql.String},
			"baseURL":       &graphql.Field{Type: graphql.String},
			"hasApiKey":     &graphql.Field{Type: graphql.Boolean},
			"apiKeyPreview": &graphql.Field{Type: graphql.String},
			"updatedAt": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					config, ok := p.Source.(domain.ModelConfigView)
					if !ok || config.UpdatedAt.IsZero() {
						return "", nil
					}
					return config.UpdatedAt.Format(time.RFC3339), nil
				},
			},
		},
	})

	ttsConfigType := graphql.NewObject(graphql.ObjectConfig{
		Name: "TTSConfig",
		Fields: graphql.Fields{
			"provider":      &graphql.Field{Type: graphql.String},
			"model":         &graphql.Field{Type: graphql.String},
			"baseURL":       &graphql.Field{Type: graphql.String},
			"voice":         &graphql.Field{Type: graphql.String},
			"audioFormat":   &graphql.Field{Type: graphql.String},
			"hasApiKey":     &graphql.Field{Type: graphql.Boolean},
			"apiKeyPreview": &graphql.Field{Type: graphql.String},
			"updatedAt": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					config, ok := p.Source.(domain.TTSConfigView)
					if !ok || config.UpdatedAt.IsZero() {
						return "", nil
					}
					return config.UpdatedAt.Format(time.RFC3339), nil
				},
			},
		},
	})

	asrConfigType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ASRConfig",
		Fields: graphql.Fields{
			"provider": &graphql.Field{Type: graphql.String}, "model": &graphql.Field{Type: graphql.String}, "baseURL": &graphql.Field{Type: graphql.String},
			"hasApiKey": &graphql.Field{Type: graphql.Boolean}, "apiKeyPreview": &graphql.Field{Type: graphql.String}, "appId": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				config, ok := p.Source.(domain.ASRConfigView)
				if !ok {
					return "", nil
				}
				return config.AppID, nil
			}},
			"updatedAt": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				config, ok := p.Source.(domain.ASRConfigView)
				if !ok || config.UpdatedAt.IsZero() {
					return "", nil
				}
				return config.UpdatedAt.Format(time.RFC3339), nil
			}},
		},
	})

	voiceProfileType := graphql.NewObject(graphql.ObjectConfig{
		Name: "VoiceProfile",
		Fields: graphql.Fields{
			"id":                &graphql.Field{Type: graphql.String},
			"name":              &graphql.Field{Type: graphql.String},
			"prompt":            &graphql.Field{Type: graphql.String},
			"language":          &graphql.Field{Type: graphql.String},
			"provider":          &graphql.Field{Type: graphql.String},
			"model":             &graphql.Field{Type: graphql.String},
			"previewAudioUrl":   &graphql.Field{Type: graphql.String},
			"status":            &graphql.Field{Type: graphql.String},
			"generationMessage": &graphql.Field{Type: graphql.String},
			"createdAt": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					profile, ok := p.Source.(domain.VoiceProfile)
					if !ok || profile.CreatedAt.IsZero() {
						return "", nil
					}
					return profile.CreatedAt.Format(time.RFC3339), nil
				},
			},
		},
	})

	authType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthPayload",
		Fields: graphql.Fields{
			"accessToken":               &graphql.Field{Type: graphql.String},
			"refreshToken":              &graphql.Field{Type: graphql.String},
			"userId":                    &graphql.Field{Type: graphql.String},
			"emailVerificationRequired": &graphql.Field{Type: graphql.Boolean},
			"emailSent":                 &graphql.Field{Type: graphql.Boolean},
			"message":                   &graphql.Field{Type: graphql.String},
		},
	})

	loginCandidateType := graphql.NewObject(graphql.ObjectConfig{
		Name: "LoginCandidate",
		Fields: graphql.Fields{
			"id":       &graphql.Field{Type: graphql.String},
			"username": &graphql.Field{Type: graphql.String},
			"email":    &graphql.Field{Type: graphql.String},
		},
	})

	emailActionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "EmailActionResult",
		Fields: graphql.Fields{
			"requiresSelection": &graphql.Field{Type: graphql.Boolean},
			"candidates":        &graphql.Field{Type: graphql.NewList(loginCandidateType)},
			"message":           &graphql.Field{Type: graphql.String},
		},
	})

	practiceResultType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PracticeResult",
		Fields: graphql.Fields{
			"score":        &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"xpEarned":     &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"feedback":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"correctCount": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"totalCount":   &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
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

	contentSourceType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ContentSource",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.String},
			"name":        &graphql.Field{Type: graphql.String},
			"domain":      &graphql.Field{Type: graphql.String},
			"category":    &graphql.Field{Type: graphql.String},
			"exam":        &graphql.Field{Type: graphql.String},
			"useCases":    &graphql.Field{Type: graphql.NewList(graphql.String)},
			"contentMode": &graphql.Field{Type: graphql.String},
			"enabled":     &graphql.Field{Type: graphql.Boolean},
			"priority":    &graphql.Field{Type: graphql.Int},
		},
	})

	readingMaterialType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ReadingMaterial",
		Fields: graphql.Fields{
			"vocabularyItems": &graphql.Field{Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "VocabularyItem",
				Fields: graphql.Fields{
					"word":     &graphql.Field{Type: graphql.String},
					"pos":      &graphql.Field{Type: graphql.String},
					"meanings": &graphql.Field{Type: graphql.NewList(graphql.String)},
				},
			}))},
			"associationSentences": &graphql.Field{Type: graphql.NewList(graphql.String)},
			"grammarInsights": &graphql.Field{Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
				Name: "GrammarInsight",
				Fields: graphql.Fields{
					"sentence":         &graphql.Field{Type: graphql.String},
					"difficultyPoints": &graphql.Field{Type: graphql.NewList(graphql.String)},
					"studySuggestions": &graphql.Field{Type: graphql.NewList(graphql.String)},
				},
			}))},
			"id":                 &graphql.Field{Type: graphql.String},
			"exam":               &graphql.Field{Type: graphql.String},
			"language":           &graphql.Field{Type: graphql.String},
			"level":              &graphql.Field{Type: graphql.String},
			"topic":              &graphql.Field{Type: graphql.String},
			"band":               &graphql.Field{Type: graphql.Float},
			"stage":              &graphql.Field{Type: graphql.String},
			"section":            &graphql.Field{Type: graphql.String},
			"skillFocus":         &graphql.Field{Type: graphql.String},
			"questionType":       &graphql.Field{Type: graphql.String},
			"scenarioFamily":     &graphql.Field{Type: graphql.String},
			"title":              &graphql.Field{Type: graphql.String},
			"passage":            &graphql.Field{Type: graphql.String},
			"vocabulary":         &graphql.Field{Type: graphql.NewList(graphql.String)},
			"sourceIds":          &graphql.Field{Type: graphql.NewList(graphql.String)},
			"generationNote":     &graphql.Field{Type: graphql.String},
			"audioUrl":           &graphql.Field{Type: graphql.String},
			"audioUrls":          &graphql.Field{Type: graphql.NewList(graphql.String)},
			"audioStatus":        &graphql.Field{Type: graphql.String},
			"status":             &graphql.Field{Type: graphql.String},
			"generationProgress": &graphql.Field{Type: graphql.Int},
			"generationMessage":  &graphql.Field{Type: graphql.String},
			"questions": &graphql.Field{
				Type: graphql.NewList(theaterQuizPublicType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					m, ok := p.Source.(domain.ReadingMaterial)
					if !ok {
						return nil, errors.New("invalid reading material source")
					}
					public := make([]map[string]interface{}, 0, len(m.Questions))
					for _, q := range m.Questions {
						options := q.Options
						if options == nil {
							options = []string{}
						}
						public = append(public, map[string]interface{}{
							"question":     q.Question,
							"options":      options,
							"answerKey":    q.AnswerKey,
							"type":         q.Type,
							"paragraphRef": q.ParagraphRef,
							"evidence":     q.Evidence,
							"headings":     q.Headings,
							"statements":   q.Statements,
							"summaryText":  q.SummaryText,
							"wordBank":     q.WordBank,
							"answers":      q.Answers,
						})
					}
					return public, nil
				},
			},
		},
	})

	roleplayType := graphql.NewObject(graphql.ObjectConfig{
		Name: "RoleplaySession",
		Fields: graphql.Fields{
			"id":                &graphql.Field{Type: graphql.String},
			"userId":            &graphql.Field{Type: graphql.String},
			"theaterId":         &graphql.Field{Type: graphql.String},
			"userRole":          &graphql.Field{Type: graphql.String},
			"turnIndex":         &graphql.Field{Type: graphql.Int},
			"currentScore":      &graphql.Field{Type: graphql.Int},
			"status":            &graphql.Field{Type: graphql.String},
			"processingMessage": &graphql.Field{Type: graphql.String},
			"finalFeedback":     &graphql.Field{Type: graphql.String},
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

	writingPromptType := graphql.NewObject(graphql.ObjectConfig{Name: "WritingPrompt", Fields: graphql.Fields{"title": &graphql.Field{Type: graphql.String}, "instructions": &graphql.Field{Type: graphql.String}, "suggestedWordCount": &graphql.Field{Type: graphql.Int}}})
	writingEvaluationType := graphql.NewObject(graphql.ObjectConfig{Name: "WritingEvaluation", Fields: graphql.Fields{"overallScore": &graphql.Field{Type: graphql.Float}, "grammarScore": &graphql.Field{Type: graphql.Float}, "vocabularyScore": &graphql.Field{Type: graphql.Float}, "coherenceScore": &graphql.Field{Type: graphql.Float}, "taskResponseScore": &graphql.Field{Type: graphql.Float}, "strengths": &graphql.Field{Type: graphql.NewList(graphql.String)}, "issues": &graphql.Field{Type: graphql.NewList(graphql.String)}, "suggestions": &graphql.Field{Type: graphql.NewList(graphql.String)}, "revisedExcerpt": &graphql.Field{Type: graphql.String}, "summary": &graphql.Field{Type: graphql.String}}})
	writingSessionType := graphql.NewObject(graphql.ObjectConfig{Name: "WritingSession", Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.String}, "exam": &graphql.Field{Type: graphql.String}, "timeLimitSeconds": &graphql.Field{Type: graphql.Int}, "prompt": &graphql.Field{Type: writingPromptType}, "essay": &graphql.Field{Type: graphql.String}, "wordCount": &graphql.Field{Type: graphql.Int}, "status": &graphql.Field{Type: graphql.String}, "progressMessage": &graphql.Field{Type: graphql.String}, "evaluation": &graphql.Field{Type: writingEvaluationType},
		"startedAt": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			item, ok := p.Source.(domain.WritingSession)
			if !ok || item.StartedAt.IsZero() {
				return "", nil
			}
			return item.StartedAt.Format(time.RFC3339), nil
		}},
		"submittedAt": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			item, ok := p.Source.(domain.WritingSession)
			if !ok || item.SubmittedAt.IsZero() {
				return "", nil
			}
			return item.SubmittedAt.Format(time.RFC3339), nil
		}},
	}})

	generateInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "GenerateTheaterInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"language":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"topic":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"difficulty":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Float)},
			"mode":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"voiceMode":       &graphql.InputObjectFieldConfig{Type: graphql.String},
			"voiceProfileIds": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		},
	})

	queryFields := graphql.Fields{
		"loginCandidates": &graphql.Field{
			Type: graphql.NewList(loginCandidateType),
			Args: graphql.FieldConfigArgument{
				"identifier": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return svc.LoginCandidates(p.Args["identifier"].(string))
			},
		},
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
		"xpEvents": &graphql.Field{Type: graphql.NewList(xpEventType), Args: graphql.FieldConfigArgument{"limit": &graphql.ArgumentConfig{Type: graphql.Int}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			limit, _ := p.Args["limit"].(int)
			return svc.XPEvents(userID, limit)
		}},
		"modelConfig": &graphql.Field{
			Type: modelConfigType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				return svc.GetModelConfig()
			},
		},
		"ttsConfig": &graphql.Field{
			Type: ttsConfigType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				return svc.GetTTSConfig()
			},
		},
		"asrConfig": &graphql.Field{Type: asrConfigType, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.GetASRConfig()
		}},
		"voiceProfiles": &graphql.Field{
			Type: graphql.NewList(voiceProfileType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				return svc.VoiceProfiles(userID)
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
		"sharedTheater": &graphql.Field{
			Type: theaterType,
			Args: graphql.FieldConfigArgument{
				"shareCode": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return svc.SharedTheater(p.Args["shareCode"].(string))
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
		"contentSources": &graphql.Field{
			Type: graphql.NewList(contentSourceType),
			Args: graphql.FieldConfigArgument{
				"exam":     &graphql.ArgumentConfig{Type: graphql.String},
				"category": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				exam, _ := p.Args["exam"].(string)
				category, _ := p.Args["category"].(string)
				return svc.ListContentSources(exam, category)
			},
		},
		"readingMaterials": &graphql.Field{
			Type: graphql.NewList(readingMaterialType),
			Args: graphql.FieldConfigArgument{
				"exam": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				exam, _ := p.Args["exam"].(string)
				return svc.ReadingMaterials(userID, exam)
			},
		},
		"readingMaterial": &graphql.Field{
			Type: readingMaterialType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				return svc.ReadingMaterial(userID, p.Args["id"].(string))
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
		"writingSession": &graphql.Field{Type: writingSessionType, Args: graphql.FieldConfigArgument{"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.WritingSession(userID, p.Args["sessionId"].(string))
		}},
		"writingSessions": &graphql.Field{Type: graphql.NewList(writingSessionType), Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.WritingSessions(userID)
		}},
	}
	if svc.CommercialFeaturesEnabled() {
		queryFields["billingProducts"] = &graphql.Field{Type: graphql.NewList(billingProductType), Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return svc.BillingProducts(), nil
		}}
		queryFields["billingStatus"] = &graphql.Field{Type: billingStatusType, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.BillingStatus(userID)
		}}
		queryFields["paymentOrder"] = &graphql.Field{Type: paymentOrderType, Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.PaymentOrder(userID, p.Args["id"].(string))
		}}
		queryFields["adPlacements"] = &graphql.Field{Type: graphql.NewList(adPlacementType), Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.AdPlacements(userID)
		}}
		queryFields["aiCreditCosts"] = &graphql.Field{Type: graphql.NewList(aiCreditCostType), Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return svc.AICreditCosts(), nil
		}}
	}
	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: queryFields})

	mutationFields := graphql.Fields{
		"register": &graphql.Field{
			Type: authType,
			Args: graphql.FieldConfigArgument{
				"username": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"email":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return svc.Register(p.Args["username"].(string), p.Args["email"].(string), p.Args["password"].(string))
			},
		},
		"login": &graphql.Field{
			Type: authType,
			Args: graphql.FieldConfigArgument{
				"identifier": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"password":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"userId":     &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Args["userId"].(string)
				return svc.Login(p.Args["identifier"].(string), p.Args["password"].(string), userID)
			},
		},
		"requestEmailVerification": &graphql.Field{
			Type: emailActionType,
			Args: graphql.FieldConfigArgument{
				"identifier": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"userId":     &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Args["userId"].(string)
				return svc.RequestEmailVerification(p.Args["identifier"].(string), userID)
			},
		},
		"verifyEmail": &graphql.Field{
			Type: authType,
			Args: graphql.FieldConfigArgument{"token": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return svc.VerifyEmail(p.Args["token"].(string))
			},
		},
		"requestPasswordReset": &graphql.Field{
			Type: emailActionType,
			Args: graphql.FieldConfigArgument{
				"identifier": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"userId":     &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Args["userId"].(string)
				return svc.RequestPasswordReset(p.Args["identifier"].(string), userID)
			},
		},
		"resetPassword": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"token":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				if err := svc.ResetPassword(p.Args["token"].(string), p.Args["password"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
		"requestUsernameRecovery": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{"email": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				if err := svc.RequestUsernameRecovery(p.Args["email"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
		"refresh": &graphql.Field{
			Type: authType,
			Args: graphql.FieldConfigArgument{
				"refreshToken": &graphql.ArgumentConfig{Type: graphql.String},
				"accessToken":  &graphql.ArgumentConfig{Type: graphql.String, Description: "Deprecated: use refreshToken instead."},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				refreshToken, _ := p.Args["refreshToken"].(string)
				accessToken, _ := p.Args["accessToken"].(string)
				var result domain.AuthResult
				var err error
				if refreshToken != "" {
					result, err = svc.Refresh(refreshToken)
				} else if accessToken != "" {
					result, err = svc.RefreshLegacyAccessToken(accessToken)
				} else {
					err = errors.New("refresh token is required")
				}
				if err != nil {
					return nil, err
				}
				return result, nil
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
		"updateModelConfig": &graphql.Field{
			Type: modelConfigType,
			Args: graphql.FieldConfigArgument{
				"provider":    &graphql.ArgumentConfig{Type: graphql.String},
				"model":       &graphql.ArgumentConfig{Type: graphql.String},
				"baseURL":     &graphql.ArgumentConfig{Type: graphql.String},
				"apiKey":      &graphql.ArgumentConfig{Type: graphql.String},
				"clearApiKey": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				provider, _ := p.Args["provider"].(string)
				model, _ := p.Args["model"].(string)
				baseURL, _ := p.Args["baseURL"].(string)
				apiKey, _ := p.Args["apiKey"].(string)
				clearAPIKey, _ := p.Args["clearApiKey"].(bool)
				return svc.UpdateModelConfig(domain.ModelConfigUpdate{
					Provider:    provider,
					Model:       model,
					BaseURL:     baseURL,
					APIKey:      apiKey,
					ClearAPIKey: clearAPIKey,
				})
			},
		},
		"updateTTSConfig": &graphql.Field{
			Type: ttsConfigType,
			Args: graphql.FieldConfigArgument{
				"provider":    &graphql.ArgumentConfig{Type: graphql.String},
				"model":       &graphql.ArgumentConfig{Type: graphql.String},
				"baseURL":     &graphql.ArgumentConfig{Type: graphql.String},
				"voice":       &graphql.ArgumentConfig{Type: graphql.String},
				"apiKey":      &graphql.ArgumentConfig{Type: graphql.String},
				"clearApiKey": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				provider, _ := p.Args["provider"].(string)
				model, _ := p.Args["model"].(string)
				baseURL, _ := p.Args["baseURL"].(string)
				voice, _ := p.Args["voice"].(string)
				apiKey, _ := p.Args["apiKey"].(string)
				clearAPIKey, _ := p.Args["clearApiKey"].(bool)
				return svc.UpdateTTSConfig(domain.TTSConfigUpdate{
					Provider:    provider,
					Model:       model,
					BaseURL:     baseURL,
					Voice:       voice,
					APIKey:      apiKey,
					ClearAPIKey: clearAPIKey,
				})
			},
		},
		"updateASRConfig": &graphql.Field{Type: asrConfigType, Args: graphql.FieldConfigArgument{
			"provider": &graphql.ArgumentConfig{Type: graphql.String}, "model": &graphql.ArgumentConfig{Type: graphql.String}, "baseURL": &graphql.ArgumentConfig{Type: graphql.String}, "apiKey": &graphql.ArgumentConfig{Type: graphql.String}, "appId": &graphql.ArgumentConfig{Type: graphql.String}, "clearApiKey": &graphql.ArgumentConfig{Type: graphql.Boolean},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			provider, _ := p.Args["provider"].(string)
			model, _ := p.Args["model"].(string)
			baseURL, _ := p.Args["baseURL"].(string)
			apiKey, _ := p.Args["apiKey"].(string)
			appID, _ := p.Args["appId"].(string)
			clearKey, _ := p.Args["clearApiKey"].(bool)
			return svc.UpdateASRConfig(domain.ASRConfigUpdate{Provider: provider, Model: model, BaseURL: baseURL, APIKey: apiKey, AppID: appID, ClearAPIKey: clearKey})
		}},
		"createVoiceProfile": &graphql.Field{
			Type: voiceProfileType,
			Args: graphql.FieldConfigArgument{
				"name":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"prompt":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"language": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				return svc.CreateVoiceProfile(userID, p.Args["name"].(string), p.Args["prompt"].(string), p.Args["language"].(string))
			},
		},
		"deleteVoiceProfile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return false, errors.New("unauthorized")
				}
				if err := svc.DeleteVoiceProfile(userID, p.Args["id"].(string)); err != nil {
					return false, err
				}
				return true, nil
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
					Language        string   `json:"language"`
					Topic           string   `json:"topic"`
					Difficulty      float64  `json:"difficulty"`
					Mode            string   `json:"mode"`
					VoiceMode       string   `json:"voiceMode"`
					VoiceProfileIDs []string `json:"voiceProfileIds"`
				}
				_ = json.Unmarshal(raw, &payload)
				return svc.GenerateTheaterWithVoices(userID, payload.Language, payload.Topic, payload.Difficulty, payload.Mode, payload.VoiceMode, payload.VoiceProfileIDs)
			},
		},
		"generateReading": &graphql.Field{
			Type: readingMaterialType,
			Args: graphql.FieldConfigArgument{
				"exam":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"topic":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"level":          &graphql.ArgumentConfig{Type: graphql.String},
				"sourceIds":      &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
				"band":           &graphql.ArgumentConfig{Type: graphql.Float},
				"stage":          &graphql.ArgumentConfig{Type: graphql.String},
				"section":        &graphql.ArgumentConfig{Type: graphql.String},
				"skillFocus":     &graphql.ArgumentConfig{Type: graphql.String},
				"questionType":   &graphql.ArgumentConfig{Type: graphql.String},
				"scenarioFamily": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return nil, errors.New("unauthorized")
				}
				sourceIDs := []string{}
				if anyIDs, ok := p.Args["sourceIds"].([]interface{}); ok {
					for _, id := range anyIDs {
						sourceIDs = append(sourceIDs, id.(string))
					}
				}
				level, _ := p.Args["level"].(string)
				band, _ := p.Args["band"].(float64)
				stage, _ := p.Args["stage"].(string)
				section, _ := p.Args["section"].(string)
				skillFocus, _ := p.Args["skillFocus"].(string)
				questionType, _ := p.Args["questionType"].(string)
				scenarioFamily, _ := p.Args["scenarioFamily"].(string)
				return svc.GenerateReadingMaterialWithInput(userID, domain.ReadingGenerationInput{
					Exam: p.Args["exam"].(string), Topic: p.Args["topic"].(string), Level: level, SourceIDs: sourceIDs,
					Band: band, Stage: stage, Section: section, SkillFocus: skillFocus, QuestionType: questionType, ScenarioFamily: scenarioFamily,
				})
			},
		},
		"deleteReadingMaterial": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"materialId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID, _ := p.Context.Value(UserIDKey).(string)
				if userID == "" {
					return false, errors.New("unauthorized")
				}
				if err := svc.DeleteReadingMaterial(userID, p.Args["materialId"].(string)); err != nil {
					return false, err
				}
				return true, nil
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
		"submitReadingAnswers": &graphql.Field{
			Type: practiceResultType,
			Args: graphql.FieldConfigArgument{
				"materialId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"answers":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.String))},
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
				return svc.SubmitReadingAnswers(userID, p.Args["materialId"].(string), answers)
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
		"submitRoleplayAudio": &graphql.Field{Type: roleplayType, Args: graphql.FieldConfigArgument{
			"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}, "audioDataUrl": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "language": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.SubmitRoleplayAudio(userID, p.Args["sessionId"].(string), p.Args["audioDataUrl"].(string), p.Args["language"].(string))
		}},
		"startWritingSession": &graphql.Field{Type: writingSessionType, Args: graphql.FieldConfigArgument{"exam": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, "timeLimitSeconds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.StartWritingSession(userID, p.Args["exam"].(string), p.Args["timeLimitSeconds"].(int))
		}},
		"submitWritingSession": &graphql.Field{Type: writingSessionType, Args: graphql.FieldConfigArgument{"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}, "essay": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			return svc.SubmitWritingSession(userID, p.Args["sessionId"].(string), p.Args["essay"].(string))
		}},
		"deleteWritingSession": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Args: graphql.FieldConfigArgument{"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return false, errors.New("unauthorized")
			}
			if err := svc.DeleteWritingSession(userID, p.Args["sessionId"].(string)); err != nil {
				return false, err
			}
			return true, nil
		}},
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
	}
	if !svc.UserServiceConfigurationEnabled() {
		delete(mutationFields, "updateModelConfig")
		delete(mutationFields, "updateTTSConfig")
		delete(mutationFields, "updateASRConfig")
	}
	if svc.SubscriptionFeaturesEnabled() {
		mutationFields["createPaymentOrder"] = &graphql.Field{Type: paymentOrderType, Args: graphql.FieldConfigArgument{
			"productCode": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"channel":     &graphql.ArgumentConfig{Type: graphql.String},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			userID, _ := p.Context.Value(UserIDKey).(string)
			if userID == "" {
				return nil, errors.New("unauthorized")
			}
			channel, _ := p.Args["channel"].(string)
			return svc.CreatePaymentOrder(userID, p.Args["productCode"].(string), channel)
		}}
	}
	mutation := graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: mutationFields})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    query,
		Mutation: mutation,
	})
}

func billingStatusTimeResolver(extract func(domain.BillingStatus) time.Time) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		status, ok := p.Source.(domain.BillingStatus)
		if !ok {
			return "", nil
		}
		value := extract(status)
		if value.IsZero() {
			return "", nil
		}
		return value.Format(time.RFC3339), nil
	}
}

func paymentOrderTimeResolver(extract func(domain.PaymentOrder) time.Time) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		order, ok := p.Source.(domain.PaymentOrder)
		if !ok {
			return "", nil
		}
		value := extract(order)
		if value.IsZero() {
			return "", nil
		}
		return value.Format(time.RFC3339), nil
	}
}

func xpEventTimeResolver(p graphql.ResolveParams) (interface{}, error) {
	event, ok := p.Source.(domain.XPEvent)
	if !ok || event.CreatedAt.IsZero() {
		return "", nil
	}
	return event.CreatedAt.Format(time.RFC3339), nil
}
