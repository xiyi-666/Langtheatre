package graph

import (
	"errors"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/service"
)

// 独立投影确保训练提交前无法请求正确答案、证据或听力原文。
func listeningTrainingView(paper domain.MockExam, summary bool) map[string]interface{} {
	view := map[string]interface{}{"id": paper.ID, "part": service.ListeningTrainingPart(paper.Exam), "status": paper.Status,
		"message": paper.CurrentSection, "createdAt": paper.CreatedAt.Format(time.RFC3339), "estimatedReadySeconds": paper.EstimatedReadySeconds,
		"generationEstimateSamples": paper.GenerationEstimateSamples, "questions": []interface{}{}, "audioUrls": []string{}, "answers": []string{}}
	if len(paper.Sections) == 0 {
		return view
	}
	section := paper.Sections[0]
	view["title"], view["targetBand"] = section.Title, section.TargetBand
	if paper.Status == "COMPLETED" && paper.Result != nil {
		view["correct"], view["total"], view["accuracy"] = paper.Result.ListeningCorrect, paper.Result.ListeningTotal, paper.Result.TotalScore
		view["feedback"], view["recommendations"] = paper.Result.Feedback, paper.Result.Recommendations
	}
	if summary || (paper.Status != "IN_PROGRESS" && paper.Status != "COMPLETED") {
		return view
	}
	view["instructions"], view["audioUrls"], view["answers"] = section.Instructions, section.AudioURLs, section.Answers
	questions := make([]map[string]interface{}, 0, len(section.Questions))
	for i, q := range section.Questions {
		options := q.Options
		if options == nil {
			options = []string{}
		}
		row := map[string]interface{}{"question": q.Question, "type": q.Type, "options": options}
		if paper.Status == "COMPLETED" {
			answer := ""
			if i < len(section.Answers) {
				answer = section.Answers[i]
			}
			row["answerKey"], row["evidence"], row["correct"] = q.AnswerKey, q.Evidence, service.ListeningAnswerMatches(answer, q)
		}
		questions = append(questions, row)
	}
	view["questions"] = questions
	if paper.Status == "COMPLETED" {
		view["transcript"] = section.AudioScript
	}
	return view
}

func registerListeningTraining(svc *service.Service, query *graphql.Object, mutations graphql.Fields) {
	questionType := graphql.NewObject(graphql.ObjectConfig{Name: "ListeningTrainingQuestion", Fields: graphql.Fields{
		"question": {Type: graphql.String}, "type": {Type: graphql.String}, "options": {Type: graphql.NewList(graphql.String)},
		"answerKey": {Type: graphql.String}, "evidence": {Type: graphql.String}, "correct": {Type: graphql.Boolean},
	}})
	trainingType := graphql.NewObject(graphql.ObjectConfig{Name: "ListeningTraining", Fields: graphql.Fields{
		"id": {Type: graphql.ID}, "part": {Type: graphql.Int}, "status": {Type: graphql.String}, "message": {Type: graphql.String},
		"title": {Type: graphql.String}, "createdAt": {Type: graphql.String}, "targetBand": {Type: graphql.Float},
		"estimatedReadySeconds": {Type: graphql.Int}, "generationEstimateSamples": {Type: graphql.Int},
		"instructions": {Type: graphql.String}, "transcript": {Type: graphql.String}, "audioUrls": {Type: graphql.NewList(graphql.String)},
		"questions": {Type: graphql.NewList(questionType)}, "answers": {Type: graphql.NewList(graphql.String)},
		"correct": {Type: graphql.Int}, "total": {Type: graphql.Int}, "accuracy": {Type: graphql.Float},
		"feedback": {Type: graphql.String}, "recommendations": {Type: graphql.NewList(graphql.String)},
	}})
	idArgs := graphql.FieldConfigArgument{"id": {Type: graphql.NewNonNull(graphql.ID)}}
	user := func(p graphql.ResolveParams) (string, error) {
		id, _ := p.Context.Value(UserIDKey).(string)
		if id == "" {
			return "", errors.New("请先登录后使用听力训练")
		}
		return id, nil
	}
	query.AddFieldConfig("listeningTrainingGenerationCost", &graphql.Field{Type: graphql.Int, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		if _, err := user(p); err != nil {
			return nil, err
		}
		return svc.ListeningTrainingGenerationCost(), nil
	}})
	query.AddFieldConfig("listeningTrainings", &graphql.Field{Type: graphql.NewList(trainingType), Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id, err := user(p)
		if err != nil {
			return nil, err
		}
		papers, err := svc.ListeningTrainings(id)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]interface{}, 0, len(papers))
		for _, paper := range papers {
			result = append(result, listeningTrainingView(paper, true))
		}
		return result, nil
	}})
	query.AddFieldConfig("listeningTraining", &graphql.Field{Type: trainingType, Args: idArgs, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id, err := user(p)
		if err != nil {
			return nil, err
		}
		paper, err := svc.ListeningTraining(id, p.Args["id"].(string))
		if err != nil {
			return nil, err
		}
		return listeningTrainingView(paper, false), nil
	}})
	mutations["startListeningTraining"] = &graphql.Field{Type: trainingType, Args: graphql.FieldConfigArgument{"part": {Type: graphql.NewNonNull(graphql.Int)}, "targetBand": {Type: graphql.Float, DefaultValue: 6.5}}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id, err := user(p)
		if err != nil {
			return nil, err
		}
		paper, err := svc.StartListeningTraining(id, p.Args["part"].(int), p.Args["targetBand"].(float64))
		if err != nil {
			return nil, err
		}
		return listeningTrainingView(paper, false), nil
	}}
	mutations["beginListeningTraining"] = &graphql.Field{Type: trainingType, Args: idArgs, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id, err := user(p)
		if err != nil {
			return nil, err
		}
		paper, err := svc.BeginListeningTraining(id, p.Args["id"].(string))
		if err != nil {
			return nil, err
		}
		return listeningTrainingView(paper, false), nil
	}}
	answerArgs := graphql.FieldConfigArgument{"id": {Type: graphql.NewNonNull(graphql.ID)}, "answers": {Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))}}
	for _, name := range []string{"saveListeningAnswers", "finishListeningTraining"} {
		name := name
		mutations[name] = &graphql.Field{Type: trainingType, Args: answerArgs, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			id, err := user(p)
			if err != nil {
				return nil, err
			}
			var paper domain.MockExam
			if name == "saveListeningAnswers" {
				paper, err = svc.SaveListeningAnswers(id, p.Args["id"].(string), graphQLStringList(p.Args["answers"]))
			} else {
				paper, err = svc.FinishListeningTraining(id, p.Args["id"].(string), graphQLStringList(p.Args["answers"]))
			}
			if err != nil {
				return nil, err
			}
			return listeningTrainingView(paper, false), nil
		}}
	}
	mutations["deleteListeningTraining"] = &graphql.Field{Type: graphql.Boolean, Args: idArgs, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
		id, err := user(p)
		if err != nil {
			return nil, err
		}
		if err = svc.DeleteListeningTraining(id, p.Args["id"].(string)); err != nil {
			return nil, err
		}
		return true, nil
	}}
}
