package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
)

var demoDifficultyBands = []string{"5.5", "6.5", "7.5"}

func parseDemoDifficulty(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err == nil && isValidDemoDifficulty(parsed) {
		return parsed
	}
	return 5.5
}

func isValidDemoDifficulty(value float64) bool {
	return value >= 4 && value <= 8 && value*2 == float64(int(value*2))
}

func (s *Service) ensureDemoDifficulty(userID string) (string, error) {
	s.demoMu.Lock()
	defer s.demoMu.Unlock()
	if !s.isDemoAccount(userID) {
		return "", errors.New("demo assignment is only available to the demo account")
	}
	if assigned, err := s.store.GetDemoAssignment(userID); err == nil {
		for _, value := range demoDifficultyBands {
			if value == assigned {
				return assigned, nil
			}
		}
	}
	index, err := rand.Int(rand.Reader, bigInt(len(demoDifficultyBands)))
	if err != nil {
		return "", err
	}
	assigned := demoDifficultyBands[index.Int64()]
	if err := s.store.SaveDemoAssignment(userID, assigned); err != nil {
		return "", err
	}
	return assigned, nil
}

func bigInt(value int) *big.Int {
	return big.NewInt(int64(value))
}

func demoReadingFixture(userID, exam, difficulty string) domain.ReadingMaterial {
	band := parseDemoDifficulty(difficulty)
	passageSuffix := ""
	if band >= 6.5 {
		passageSuffix = " At this level, readers should distinguish the author's main claim from supporting detail and notice how contrast markers connect ideas."
	}
	if band >= 7.5 {
		passageSuffix += " They should also evaluate implied assumptions rather than relying only on explicitly stated facts."
	}
	if strings.EqualFold(exam, "CET") {
		return domain.ReadingMaterial{
			ID: uuid.NewString(), UserID: userID, Exam: "CET", Language: "ENGLISH", Level: "intermediate", Band: band,
			Stage: fmt.Sprintf("Stage %.1f", band), Section: "Reading", SkillFocus: "main idea and detail", QuestionType: "multiple choice", ScenarioFamily: "campus life",
			Title: fmt.Sprintf("CET Reading Demo · Campus volunteering · %.1f", band), Topic: "Campus volunteering and community ties",
			Passage:    "Many universities now encourage students to join local volunteering projects. A regular commitment helps students understand community needs, practise teamwork, and connect classroom knowledge with real situations. The most successful projects begin with a clear goal and give volunteers enough guidance to work independently. When students reflect on what they have learned, volunteering becomes more than a short activity; it becomes a practical part of their education." + passageSuffix,
			Vocabulary: []string{"commitment", "encourage", "independently", "reflection"}, Questions: demoReadingQuestions("CET"),
			GenerationNote: "演示内容已预置，无需调用 AI", AudioStatus: "READY", Status: "READY", GenerationProgress: 100, GenerationMessage: "演示内容已准备，无需调用 AI", VocabularyItems: demoVocabulary(), AssociationSentences: []string{"A clear goal helps volunteers work independently.", "Reflection turns an activity into a learning experience."}, GrammarInsights: demoGrammar(), CreatedAt: time.Now().UTC(),
		}
	}
	return domain.ReadingMaterial{
		ID: uuid.NewString(), UserID: userID, Exam: "IELTS", Language: "ENGLISH", Level: "upper-intermediate", Band: band,
		Stage: fmt.Sprintf("Band %.1f", band), Section: "Reading", SkillFocus: "inference and writer's view", QuestionType: "multiple choice", ScenarioFamily: "urban life",
		Title: fmt.Sprintf("IELTS Reading Demo · Greener city transport · %.1f", band), Topic: "Greener city transport",
		Passage:    "As cities grow, transport planners are looking for ways to reduce congestion without limiting access to work and education. Expanding rail networks can move large numbers of passengers, but construction is expensive and may disrupt established neighbourhoods. Some cities have therefore combined reliable public transport with safer cycling routes and flexible ticketing. The evidence suggests that no single measure solves every problem; lasting improvement depends on connecting different options and making them convenient for everyday travellers." + passageSuffix,
		Vocabulary: []string{"congestion", "established", "flexible", "convenient"}, Questions: demoReadingQuestions("IELTS"),
		GenerationNote: "演示内容已预置，无需调用 AI", AudioStatus: "READY", Status: "READY", GenerationProgress: 100, GenerationMessage: "演示内容已准备，无需调用 AI", VocabularyItems: demoVocabulary(), AssociationSentences: []string{"No single measure solves every transport problem.", "Convenient options are more likely to become everyday habits."}, GrammarInsights: demoGrammar(), CreatedAt: time.Now().UTC(),
	}
}

func demoReadingQuestions(exam string) []domain.QuizQuestion {
	if exam == "CET" {
		return []domain.QuizQuestion{{Question: "What does volunteering help students practise?", Options: []string{"Teamwork", "Driving", "Programming", "Cooking"}, AnswerKey: "Teamwork"}, {Question: "What should successful projects provide?", Options: []string{"No goal", "Enough guidance", "Long holidays", "Private offices"}, AnswerKey: "Enough guidance"}, {Question: "What makes volunteering part of education?", Options: []string{"Reflection", "Competition", "Silence", "Travel"}, AnswerKey: "Reflection"}}
	}
	return []domain.QuizQuestion{{Question: "Why can rail construction be difficult?", Options: []string{"It is always free", "It may disrupt neighbourhoods", "It reduces access", "It needs no planning"}, AnswerKey: "It may disrupt neighbourhoods"}, {Question: "What have some cities combined with public transport?", Options: []string{"Cycling routes", "Fewer tickets", "More congestion", "Closed stations"}, AnswerKey: "Cycling routes"}, {Question: "What is the writer's main view?", Options: []string{"One measure solves everything", "Different options should connect", "Cycling should replace all transport", "Construction is unnecessary"}, AnswerKey: "Different options should connect"}}
}

func demoVocabulary() []domain.VocabularyItem {
	return []domain.VocabularyItem{{Word: "convenient", POS: "adj.", Meanings: []string{"方便的"}}, {Word: "commitment", POS: "n.", Meanings: []string{"承诺；投入"}}, {Word: "reflection", POS: "n.", Meanings: []string{"反思"}}}
}

func demoGrammar() []domain.GrammarInsight {
	return []domain.GrammarInsight{{Sentence: "lasting improvement depends on connecting different options", DifficultyPoints: []string{"depend on + gerund"}, StudySuggestions: []string{"注意 depend on 后接名词或动名词。"}}}
}

func (s *Service) ensureDemoLearningFixtures(userID, _ string) error {
	for _, difficulty := range demoDifficultyBands {
		for _, exam := range []string{"IELTS", "CET"} {
			items, err := s.store.ListReadingMaterialsByUser(userID, exam)
			if err != nil {
				return err
			}
			fixture := demoReadingFixture(userID, exam, difficulty)
			found := false
			for _, item := range items {
				if item.Band == fixture.Band && item.Topic == fixture.Topic {
					found = true
					break
				}
			}
			if !found {
				if _, err := s.store.SaveReadingMaterial(fixture); err != nil {
					return err
				}
			}
		}
	}
	for _, difficulty := range demoDifficultyBands {
		for _, exam := range []string{"IELTS", "CET4", "CET6"} {
			sessions, err := s.store.ListWritingSessions(userID)
			if err != nil {
				return err
			}
			found := false
			prompt := demoWritingPrompt(exam, difficulty)
			for _, item := range sessions {
				if item.Exam == exam && item.Prompt.Title == prompt.Title {
					found = true
					break
				}
			}
			if found {
				continue
			}
			sample := "Public transport can improve urban life because it reduces traffic and gives more people access to work and education. However, cities should also make services reliable and affordable. A balanced plan can encourage residents to choose public transport while keeping neighbourhoods connected."
			if _, err := s.store.SaveWritingSession(domain.WritingSession{ID: uuid.NewString(), UserID: userID, Exam: exam, TimeLimitSeconds: 2400, Prompt: prompt, Essay: sample, WordCount: len(strings.Fields(sample)), Status: "COMPLETED", ProgressMessage: "演示评分已准备", Evaluation: demoWritingEvaluation(), StartedAt: time.Now().UTC().Add(-30 * time.Minute), SubmittedAt: time.Now().UTC().Add(-5 * time.Minute), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
				return err
			}
		}
	}
	return nil
}

func demoWritingPrompt(exam, difficulty string) domain.WritingPrompt {
	band := parseDemoDifficulty(difficulty)
	difficultyInstruction := ""
	if band >= 6.5 {
		difficultyInstruction = " Support each main point with a specific example and use clear linking devices."
	}
	if band >= 7.5 {
		difficultyInstruction += " Address a counterargument and use precise academic vocabulary."
	}
	if exam != "IELTS" {
		return domain.WritingPrompt{Title: fmt.Sprintf("%s 演示写作 · %.1f", exam, band), Instructions: "Write an English essay about one practical change that would improve student life. Explain the problem, your proposal, and its benefits." + difficultyInstruction, SuggestedWordCount: 160}
	}
	return domain.WritingPrompt{Title: fmt.Sprintf("IELTS 演示写作 · Urban transport · %.1f", band), Instructions: "Some people think cities should invest more in public transport, while others prefer roads for private cars. Discuss both views and give your own opinion. Write in English." + difficultyInstruction, SuggestedWordCount: 250}
}

func demoWritingEvaluation() *domain.WritingEvaluation {
	return &domain.WritingEvaluation{OverallScore: 78, GrammarScore: 76, VocabularyScore: 79, CoherenceScore: 80, TaskResponseScore: 78, Strengths: []string{"观点明确，能够回应题目要求。"}, Issues: []string{"部分句子还可以加入更具体的例子。"}, Suggestions: []string{"使用让步结构连接两种观点。", "为每个主要观点补充一个真实场景。"}, RevisedExcerpt: "A balanced transport policy can improve access while reducing congestion.", Summary: "演示评分已预置，不调用 AI。"}
}

func normalizeDemoDifficulty(value string) string {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err == nil && isValidDemoDifficulty(parsed) {
		return fmt.Sprintf("%.1f", parsed)
	}
	return ""
}

func (s *Service) generateDemoReading(userID string, input domain.ReadingGenerationInput) (domain.ReadingMaterial, error) {
	difficulty, err := s.ensureDemoDifficulty(userID)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	if isValidDemoDifficulty(input.Band) {
		difficulty = fmt.Sprintf("%.1f", input.Band)
	}
	exam := strings.ToUpper(strings.TrimSpace(input.Exam))
	if exam != "CET" {
		exam = "IELTS"
	}
	fixture := demoReadingFixture(userID, exam, difficulty)
	fixture.ID = uuid.NewString()
	fixture.Stage = strings.TrimSpace(input.Stage)
	if fixture.Stage == "" {
		fixture.Stage = fmt.Sprintf("Band %.1f", fixture.Band)
	}
	fixture.CreatedAt = time.Now().UTC()
	return s.store.SaveReadingMaterial(fixture)
}

func (s *Service) generateDemoWriting(userID, exam string, timeLimitSeconds int, requestedDifficulty string) (domain.WritingSession, error) {
	difficulty, err := s.ensureDemoDifficulty(userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if normalized := normalizeDemoDifficulty(requestedDifficulty); normalized != "" {
		difficulty = normalized
	}
	now := time.Now().UTC()
	session := domain.WritingSession{ID: uuid.NewString(), UserID: userID, Exam: exam, TimeLimitSeconds: timeLimitSeconds, Prompt: demoWritingPrompt(exam, difficulty), Status: "WRITING", ProgressMessage: "演示题目已准备，提交后使用预置评分", StartedAt: now, CreatedAt: now, UpdatedAt: now}
	return s.store.SaveWritingSession(session)
}

func (s *Service) submitDemoWriting(userID, sessionID, essay string) (domain.WritingSession, error) {
	session, err := s.store.GetWritingSession(sessionID, userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if session.Status != "WRITING" {
		return domain.WritingSession{}, errors.New("writing has already been submitted")
	}
	if len([]rune(essay)) < 30 {
		return domain.WritingSession{}, errors.New("essay must contain at least 30 characters")
	}
	session.Essay = strings.TrimSpace(essay)
	session.WordCount = len(strings.Fields(session.Essay))
	session.SubmittedAt = time.Now().UTC()
	session.Status = "COMPLETED"
	session.ProgressMessage = "演示评分完成"
	evaluation := *demoWritingEvaluation()
	evaluation.OverallScore = float64(min(95, max(55, 50+session.WordCount/3)))
	session.Evaluation = &evaluation
	saved, err := s.store.UpdateWritingSessionExisting(session)
	if err == nil {
		_, _ = s.awardLearningXP(userID, "WRITING_COMPLETE", sessionID, int(evaluation.OverallScore))
	}
	return saved, err
}
