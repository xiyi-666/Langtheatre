package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/linguaquest/server/internal/config"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/store"
	_ "modernc.org/sqlite"
)

// 此数据仅测试结构、长度和协议，不能作为生成失败时的备用试卷。
var mockExamFixtureParagraphs = []string{
	"The regional planning group began its investigation with a detailed survey of public spaces around the river. Residents were invited to describe how they used the paths during different seasons and explain what prevented them from visiting more frequently. Rather than assuming that everyone shared the same priorities, researchers recorded contrasting views about safety, convenience, and the protection of wildlife. Interviews revealed several practical concerns which had not appeared in the original proposal, particularly among people who travelled with children or needed assistance when walking.",
	"Historical records offered another perspective on the landscape. Maps from the municipal archive showed that small industrial buildings had once occupied much of the eastern bank, while photographs documented gradual changes in vegetation. The team compared these sources with measurements taken during recent field visits. Although some boundaries could be identified precisely, others remained uncertain because earlier surveys had used inconsistent reference points. This uncertainty influenced the decision to preserve several features until their relationship with the surrounding area could be investigated more thoroughly.",
	"Financial constraints also shaped the available choices. An independent assessment compared the initial construction budget with likely maintenance expenses over the following decade. Materials that appeared economical at first sometimes required specialist repairs or frequent replacement, making them less attractive when considered over a longer period. Local contractors suggested several alternatives, but the committee requested further evidence before accepting their estimates. Funding was eventually divided between essential improvements and a smaller experimental programme intended to evaluate approaches that had not previously been tried locally.",
	"Community workshops provided an opportunity to challenge the emerging design. Participants worked in small groups, using photographs and large printed plans to identify potential conflicts between activities. Cyclists wanted uninterrupted routes, whereas older residents preferred quieter places where they could rest without feeling hurried. These differences did not produce a single compromise immediately. Instead, the facilitators asked each group to explain its reasoning and consider circumstances in which another arrangement might work better. The resulting proposals were more varied than the original consultation responses suggested.",
	"Environmental monitoring continued while the consultation was taking place. Volunteers recorded bird activity at fixed observation points and noted how rainfall affected the shallow pools near the southern boundary. Specialists checked a selection of these observations to estimate their reliability. They found that training improved agreement between observers, although poor visibility still caused difficulties on some mornings. The findings encouraged the project managers to revise the monitoring schedule and distinguish short term changes from patterns that might indicate a lasting alteration in habitat quality.",
	"Communication became particularly important when construction arrangements were announced. Earlier notices had described the overall ambition but provided little information about temporary closures or alternative access. A revised information campaign therefore explained the sequence of work and gave residents a clear contact point for practical enquiries. Staff also visited nearby businesses to discuss deliveries and customer access. This approach required additional preparation, yet it reduced confusion during the busiest stage and helped the organisers identify problems before they disrupted essential local services or scheduled activities.",
	"The final evaluation considered both measurable outcomes and the experience of those involved. Visitor counts indicated a substantial increase in use, but interviews suggested that the distribution of benefits was uneven. Some residents welcomed the additional facilities, while others remained concerned about crowding during weekends. The report recommended continued observation rather than treating the opening ceremony as the end of the process. Its authors argued that successful management would depend on responding to new evidence and maintaining the relationships established during the planning and consultation stages.",
}

var mockExamFixtureFeatures = []string{"entrance", "workshop", "archive", "shelter", "platform", "garden", "footbridge", "laboratory", "gallery", "terrace", "courtyard", "pavilion", "classroom", "observatory"}
var mockExamFixtureAnswers = []string{"oak", "copper", "slate", "granite", "bamboo", "cedar", "steel", "clay", "marble", "glass", "brick", "limestone", "cork", "aluminium"}

func TestNormalizeCandidateQuestionNumbers(t *testing.T) {
	draft := &mockExamDraft{Questions: []domain.QuizQuestion{
		{Question: "1. First question?"},
		{Question: "Second question?"},
		{Question: "Question 7: Third question?"},
	}}
	if got := normalizeCandidateQuestionNumbers(draft); got != 2 {
		t.Fatalf("changed=%d, want 2", got)
	}
	want := []string{"1. First question?", "2. Second question?", "3. Third question?"}
	for i, question := range draft.Questions {
		if question.Question != want[i] {
			t.Fatalf("question %d=%q, want %q", i+1, question.Question, want[i])
		}
	}
}

func mockExamFixtureSection(index int, band float64) domain.MockExamSection {
	spec := mockExamSpecs[index]
	section := domain.MockExamSection{Key: spec.key, Skill: spec.skill, Title: "River planning " + spec.key, Instructions: "Read or listen to the complete source and answer each question using the specified format.", DurationSeconds: spec.seconds, PaperVersion: mockExamPaperVersion, TargetBand: band, QualityApproved: true}
	if spec.skill == "WRITING" {
		section.WritingPrompts = []domain.WritingPrompt{
			{Title: "Task 1: Travel to work", SuggestedWordCount: 150, Instructions: "The table gives the percentage of commuters using three forms of transport in the city of Norchester in 2000 and 2020. Summarise the information by selecting and reporting the main features, and make comparisons where relevant. Use the figures in the table to support your description of the changes over time. Write at least 150 words.\n| Transport | 2000 (%) | 2020 (%) |\n| --- | --- | --- |\n| Bus | 30 | 25 |\n| Bicycle | 10 | 20 |\n| Car | 60 | 55 |"},
			{Title: "Task 2: Public investment", SuggestedWordCount: 250, Instructions: "Some people believe local governments should invest more money in public parks, while others argue that improving transport is a greater priority. Discuss both views and give your own opinion. Give reasons for your answer and include relevant examples from your knowledge or experience. Write at least 250 words."},
		}
		return section
	}
	slug := strings.ReplaceAll(strings.ToLower(spec.key), "_", "")
	facts := make([]string, 14)
	for i := range facts {
		facts[i] = fmt.Sprintf("For the %s %s, researchers selected %s after comparing several suitable materials.", slug, mockExamFixtureFeatures[i], mockExamFixtureAnswers[i])
	}
	var paragraphs []string
	for i, paragraph := range mockExamFixtureParagraphs {
		label := fmt.Sprintf("%c. ", 'A'+i)
		if spec.skill == "LISTENING" {
			label = "Maya: "
			if (index == 0 || index == 2) && i%2 == 1 {
				label = "Daniel: "
			}
		}
		paragraphs = append(paragraphs, label+paragraph+" "+facts[i*2]+" "+facts[i*2+1])
	}
	source := strings.Join(paragraphs, "\n\n")
	if spec.skill == "READING" {
		section.Passage = source
	} else {
		section.AudioScript = source
	}
	for i := 0; i < spec.questions; i++ {
		q := domain.QuizQuestion{Type: "sentence_completion", Question: fmt.Sprintf("For the %s %s, the selected material was ____. Write NO MORE THAN THREE WORDS.", slug, mockExamFixtureFeatures[i]), AnswerKey: mockExamFixtureAnswers[i], Evidence: facts[i]}
		switch {
		case i%4 == 0:
			q.Type = "multiple_choice"
			q.Question = fmt.Sprintf("Which material was selected for the %s %s?", slug, mockExamFixtureFeatures[i])
			q.Options = []string{"A. " + mockExamFixtureAnswers[i], "B. recycled paper", "C. woven fabric", "D. plastic panels"}
			q.AnswerKey = "A"
		case spec.skill == "READING" && i%4 == 2:
			q.Type = "true_false_not_given"
			q.Question = fmt.Sprintf("Researchers selected %s for the %s %s.", mockExamFixtureAnswers[i], slug, mockExamFixtureFeatures[i])
			q.Options = []string{"TRUE", "FALSE", "NOT GIVEN"}
			q.AnswerKey = "TRUE"
		case spec.skill == "READING" && i%4 == 3:
			q.Type = "matching_information"
			q.Question = fmt.Sprintf("Which paragraph describes selecting materials for the %s %s?", slug, mockExamFixtureFeatures[i])
			for j := 0; j < 7; j++ {
				q.Options = append(q.Options, fmt.Sprintf("%c. Paragraph %c", 'A'+j, 'A'+j))
			}
			q.AnswerKey = string(rune('A' + i/2))
		}
		section.Questions = append(section.Questions, q)
	}
	return section
}

func mockExamFixturePaper(band float64) domain.MockExam {
	paper := domain.MockExam{Exam: "IELTS", TotalDurationSeconds: 9000}
	for i := range mockExamSpecs {
		paper.Sections = append(paper.Sections, mockExamFixtureSection(i, band))
	}
	return paper
}

func mockExamFixtureReview(section domain.MockExamSection) mockExamReview {
	review := mockExamReview{Approved: true, Original: true, FormCorrect: true, Suitable: true, MixedDifficulty: true, NoOfficialClaims: true, Feedback: "The tasks develop varied academic comprehension skills with sufficient contextual support."}
	for i, q := range section.Questions {
		review.Answers = append(review.Answers, mockExamReviewAnswer{Number: i + 1, Answer: mockExamCanonicalAnswer(q, q.AnswerKey), Unique: true, Supported: true, Evidence: q.Evidence, Feedback: "The quoted detail provides one unambiguous answer to this question."})
	}
	if section.Skill == "WRITING" {
		// 经 JSON 构造匿名结构，保持模型返回协议与生产结构一致。
		_ = json.Unmarshal([]byte(`{"writingTasks":[{"number":1,"selfContained":true,"suitable":true,"feedback":"The numeric table supports meaningful comparisons and an overview."},{"number":2,"selfContained":true,"suitable":true,"feedback":"The essay question invites a reasoned discussion of competing priorities."}]}`), &review)
	}
	return review
}

func TestMockExamValidPaper(t *testing.T) {
	for _, band := range []float64{6, 6.5, 7, 7.5, 8} {
		if err := ValidateMockExamPaper(mockExamFixturePaper(band)); err != nil {
			t.Fatalf("band %.1f: %v", band, err)
		}
	}
	paper := mockExamFixturePaper(7)
	paper.Sections[0].Instructions += " This is not an official IELTS test. Results are not guaranteed scores."
	if err := ValidateMockExamPaper(paper); err != nil {
		t.Fatalf("valid disclaimer rejected: %v", err)
	}
}

func TestMockExamGateRejectsInvalidPaper(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.MockExam)
	}{
		{"missing section", func(p *domain.MockExam) { p.Sections = p.Sections[:7] }},
		{"wrong order", func(p *domain.MockExam) { p.Sections[0], p.Sections[4] = p.Sections[4], p.Sections[0] }},
		{"wrong exam", func(p *domain.MockExam) { p.Exam = "GENERAL" }},
		{"unapproved", func(p *domain.MockExam) { p.Sections[2].QualityApproved = false }},
		{"wrong version", func(p *domain.MockExam) { p.Sections[2].PaperVersion = "IELTS-v1" }},
		{"invalid band", func(p *domain.MockExam) { p.Sections[0].TargetBand = 6.2 }},
		{"inconsistent band", func(p *domain.MockExam) { p.Sections[4].TargetBand = 8 }},
		{"nan band", func(p *domain.MockExam) { p.Sections[0].TargetBand = math.NaN() }},
		{"empty title", func(p *domain.MockExam) { p.Sections[0].Title = " " }},
		{"empty instructions", func(p *domain.MockExam) { p.Sections[0].Instructions = " " }},
		{"wrong skill", func(p *domain.MockExam) { p.Sections[0].Skill = "READING" }},
		{"wrong duration", func(p *domain.MockExam) { p.TotalDurationSeconds = 1 }},
		{"missing questions", func(p *domain.MockExam) { p.Sections[0].Questions = p.Sections[0].Questions[:9] }},
		{"too many questions", func(p *domain.MockExam) {
			p.Sections[0].Questions = append(p.Sections[0].Questions, p.Sections[0].Questions[0])
		}},
		{"short script", func(p *domain.MockExam) { p.Sections[0].AudioScript = "Maya: This is only a short excerpt." }},
		{"long script", func(p *domain.MockExam) { p.Sections[0].AudioScript += strings.Repeat(" more", 900) }},
		{"short reading", func(p *domain.MockExam) { p.Sections[4].Passage = "A short source." }},
		{"long reading", func(p *domain.MockExam) { p.Sections[4].Passage += strings.Repeat(" more", 900) }},
		{"placeholder", func(p *domain.MockExam) { p.Sections[0].AudioScript += " The full recording will be generated later." }},
		{"ellipsis", func(p *domain.MockExam) { p.Sections[4].Passage += " ..." }},
		{"repeated filler", func(p *domain.MockExam) {
			p.Sections[0].AudioScript += "\nThe park was designed for many different local activities.\nThe park was designed for many different local activities."
		}},
		{"numbered filler", func(p *domain.MockExam) {
			p.Sections[0].AudioScript += "\n1. People gathered at the entrance to hear announcement 1.\n2. People gathered at the entrance to hear announcement 2."
		}},
		{"official claim", func(p *domain.MockExam) { p.Sections[0].Instructions += " This is an official IELTS test." }},
		{"wrong listening form", func(p *domain.MockExam) {
			p.Sections[0].AudioScript = strings.ReplaceAll(p.Sections[0].AudioScript, "Daniel:", "Maya:")
		}},
		{"monologue has two speakers", func(p *domain.MockExam) {
			p.Sections[1].AudioScript = strings.Replace(p.Sections[1].AudioScript, "Maya:", "Daniel:", 1)
		}},
		{"source in wrong field", func(p *domain.MockExam) { p.Sections[0].Passage = p.Sections[0].AudioScript }},
		{"reading with script", func(p *domain.MockExam) { p.Sections[4].AudioScript = "unexpected" }},
		{"duplicate cross paper", func(p *domain.MockExam) { p.Sections[1].Questions[0].Question = p.Sections[0].Questions[0].Question }},
		{"duplicate source", func(p *domain.MockExam) { p.Sections[6].Passage = p.Sections[5].Passage }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paper := mockExamFixturePaper(7)
			tc.mutate(&paper)
			if err := ValidateMockExamPaper(paper); err == nil {
				t.Fatal("invalid paper accepted")
			}
		})
	}
}

func TestMockExamPlaceholderDetectionDoesNotRejectNaturalCompletion(t *testing.T) {
	if mockExamPlaceholders.MatchString("The main test must be completed before noon.") {
		t.Fatal("natural scheduling statement was treated as placeholder content")
	}
	for _, value := range []string{"The full recording will be generated later.", "The passage is to be completed.", "Insert question here."} {
		if !mockExamPlaceholders.MatchString(value) {
			t.Fatalf("placeholder content accepted: %q", value)
		}
	}
}

func TestMockExamGateRejectsInvalidQuestions(t *testing.T) {
	cases := []struct {
		name   string
		index  int
		mutate func(*domain.QuizQuestion)
	}{
		{"empty question", 0, func(q *domain.QuizQuestion) { q.Question = " " }},
		{"empty key", 0, func(q *domain.QuizQuestion) { q.AnswerKey = " " }},
		{"empty evidence", 0, func(q *domain.QuizQuestion) { q.Evidence = " " }},
		{"unrelated evidence", 0, func(q *domain.QuizQuestion) { q.Evidence = "The spacecraft reached another distant planet yesterday." }},
		{"short evidence", 0, func(q *domain.QuizQuestion) { q.Evidence = "researchers" }},
		{"case changed evidence", 0, func(q *domain.QuizQuestion) { q.Evidence = strings.ToUpper(q.Evidence) }},
		{"unsupported type", 0, func(q *domain.QuizQuestion) { q.Type = "essay" }},
		{"missing options", 0, func(q *domain.QuizQuestion) { q.Options = nil }},
		{"extra options", 0, func(q *domain.QuizQuestion) { q.Options = append(q.Options, "E. rubber") }},
		{"bare letters", 0, func(q *domain.QuizQuestion) { q.Options = []string{"A", "B", "C", "D"} }},
		{"duplicate option body", 0, func(q *domain.QuizQuestion) { q.Options[1] = "B. " + strings.TrimPrefix(q.Options[0], "A. ") }},
		{"wrong letters", 0, func(q *domain.QuizQuestion) { q.Options[1] = "C. recycled paper" }},
		{"invalid key", 0, func(q *domain.QuizQuestion) { q.AnswerKey = "E" }},
		{"alternate nested answers", 0, func(q *domain.QuizQuestion) { q.Answers = []string{"B"} }},
		{"nested statements", 0, func(q *domain.QuizQuestion) {
			q.Statements = []domain.ReadingStatement{{Text: "hidden", Answer: "TRUE"}}
		}},
		{"completion options", 1, func(q *domain.QuizQuestion) { q.Options = []string{"copper"} }},
		{"completion no limit", 1, func(q *domain.QuizQuestion) { q.Question = "For the workshop, the selected material was ____." }},
		{"completion excessive limit", 1, func(q *domain.QuizQuestion) { q.Question = strings.ReplaceAll(q.Question, "THREE", "FOUR") }},
		{"completion no blank", 1, func(q *domain.QuizQuestion) { q.Question = strings.ReplaceAll(q.Question, "____", "the material") }},
		{"completion two blanks", 1, func(q *domain.QuizQuestion) { q.Question += " And ____." }},
		{"completion too long", 1, func(q *domain.QuizQuestion) { q.AnswerKey = "after comparing several suitable materials" }},
		{"completion absent answer", 1, func(q *domain.QuizQuestion) { q.AnswerKey = "astronaut" }},
		{"completion substring", 1, func(q *domain.QuizQuestion) { q.AnswerKey = "opp" }},
		{"completion alternatives", 1, func(q *domain.QuizQuestion) { q.AnswerKey = "copper/copper" }},
		{"tfng short key", 2, func(q *domain.QuizQuestion) { q.AnswerKey = "T" }},
		{"tfng wrong options", 2, func(q *domain.QuizQuestion) { q.Options = []string{"YES", "NO", "NOT GIVEN"} }},
		{"matching absent context", 3, func(q *domain.QuizQuestion) { q.Options[0] = "A. Missing visitor centre" }},
		{"matching absent paragraph", 3, func(q *domain.QuizQuestion) { q.Options[0] = "A. Paragraph H" }},
		{"matching wrong paragraph", 3, func(q *domain.QuizQuestion) { q.AnswerKey = "G" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paper := mockExamFixturePaper(7)
			tc.mutate(&paper.Sections[4].Questions[tc.index])
			if err := ValidateMockExamPaper(paper); err == nil {
				t.Fatal("invalid question accepted")
			}
		})
	}
	t.Run("duplicate ignores numbering and punctuation", func(t *testing.T) {
		p := mockExamFixturePaper(7)
		p.Sections[0].Questions[1].Question = "Question 2: " + p.Sections[0].Questions[0].Question + "!"
		if ValidateMockExamPaper(p) == nil {
			t.Fatal("duplicate accepted")
		}
	})
	t.Run("completion lower word limit", func(t *testing.T) {
		q := mockExamFixtureSection(0, 7).Questions[1]
		q.Question = strings.ReplaceAll(q.Question, "THREE WORDS", "ONE WORD")
		q.AnswerKey = "suitable materials"
		if validateMockExamQuestion(q, mockExamFixtureSection(0, 7).AudioScript, "LISTENING") == nil {
			t.Fatal("over limit answer accepted")
		}
	})
}

func TestMockExamWritingGate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.MockExamSection)
	}{
		{"missing task", func(s *domain.MockExamSection) { s.WritingPrompts = s.WritingPrompts[:1] }},
		{"wrong task order", func(s *domain.MockExamSection) {
			s.WritingPrompts[0], s.WritingPrompts[1] = s.WritingPrompts[1], s.WritingPrompts[0]
		}},
		{"wrong word target", func(s *domain.MockExamSection) { s.WritingPrompts[0].SuggestedWordCount = 100 }},
		{"missing explicit requirement", func(s *domain.MockExamSection) {
			s.WritingPrompts[1].Instructions = strings.ReplaceAll(s.WritingPrompts[1].Instructions, "250", "300")
		}},
		{"missing table", func(s *domain.MockExamSection) {
			s.WritingPrompts[0].Instructions = strings.Split(s.WritingPrompts[0].Instructions, "\n")[0]
		}},
		{"non numeric", func(s *domain.MockExamSection) {
			s.WritingPrompts[0].Instructions = strings.ReplaceAll(s.WritingPrompts[0].Instructions, "| 30 |", "| unknown |")
		}},
		{"missing cell", func(s *domain.MockExamSection) {
			s.WritingPrompts[0].Instructions = strings.ReplaceAll(s.WritingPrompts[0].Instructions, "| 30 | 25 |", "| 30 |")
		}},
		{"duplicate row", func(s *domain.MockExamSection) {
			s.WritingPrompts[0].Instructions = strings.ReplaceAll(s.WritingPrompts[0].Instructions, "| Bicycle |", "| Bus |")
		}},
		{"missing units", func(s *domain.MockExamSection) {
			s.WritingPrompts[0].Instructions = strings.NewReplacer("percentage", "proportion", "%", "").Replace(s.WritingPrompts[0].Instructions)
		}},
		{"task2 missing visual", func(s *domain.MockExamSection) { s.WritingPrompts[1].Instructions += " Refer to the chart below." }},
		{"unexpected source", func(s *domain.MockExamSection) { s.Passage = "The answer is here." }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := mockExamFixturePaper(7)
			tc.mutate(&p.Sections[7])
			if ValidateMockExamPaper(p) == nil {
				t.Fatal("invalid writing accepted")
			}
		})
	}
}

func TestMockExamReviewGate(t *testing.T) {
	section := mockExamFixtureSection(4, 7)
	if err := validateMockExamReview(section, mockExamFixtureReview(section)); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*mockExamReview)
	}{
		{"not approved", func(r *mockExamReview) { r.Approved = false }},
		{"not original", func(r *mockExamReview) { r.Original = false }},
		{"wrong form", func(r *mockExamReview) { r.FormCorrect = false }},
		{"not suitable", func(r *mockExamReview) { r.Suitable = false }},
		{"flat difficulty", func(r *mockExamReview) { r.MixedDifficulty = false }},
		{"official claims", func(r *mockExamReview) { r.NoOfficialClaims = false }},
		{"no feedback", func(r *mockExamReview) { r.Feedback = "" }},
		{"missing answers", func(r *mockExamReview) { r.Answers = r.Answers[:1] }},
		{"duplicate numbers", func(r *mockExamReview) { r.Answers[1].Number = 1 }},
		{"out of range", func(r *mockExamReview) { r.Answers[0].Number = 99 }},
		{"not unique", func(r *mockExamReview) { r.Answers[0].Unique = false }},
		{"unsupported", func(r *mockExamReview) { r.Answers[0].Supported = false }},
		{"wrong answer", func(r *mockExamReview) { r.Answers[0].Answer = "B" }},
		{"unrelated evidence", func(r *mockExamReview) { r.Answers[0].Evidence = "A spaceship landed outside the distant town." }},
		{"no per item feedback", func(r *mockExamReview) { r.Answers[0].Feedback = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := mockExamFixtureReview(section)
			tc.mutate(&r)
			if validateMockExamReview(section, r) == nil {
				t.Fatal("invalid review accepted")
			}
		})
	}
	t.Run("writing review required", func(t *testing.T) {
		s := mockExamFixtureSection(7, 7)
		r := mockExamFixtureReview(s)
		if err := validateMockExamReview(s, r); err != nil {
			t.Fatal(err)
		}
		r.WritingTasks[1].Number = 1
		if validateMockExamReview(s, r) == nil {
			t.Fatal("duplicate task review accepted")
		}
	})
}

func TestMockExamListeningMatchingAllowsParaphrase(t *testing.T) {
	section := mockExamFixtureSection(0, 7)
	q := &section.Questions[0]
	q.Type = "matching_information"
	q.Options = []string{"A. A type of hardwood", "B. A lightweight synthetic material", "C. A naturally occurring mineral"}
	q.AnswerKey = "A"
	if err := validateMockExamSection(section, mockExamSpecs[0]); err != nil {
		t.Fatalf("valid listening paraphrase rejected: %v", err)
	}
	review := mockExamFixtureReview(section)
	if err := validateMockExamReview(section, review); err != nil {
		t.Fatal(err)
	}
	review.Answers[0].Answer = "B"
	if err := validateMockExamReview(section, review); err == nil {
		t.Fatal("paraphrase relaxation must not bypass independent answer matching")
	}
	q.Options[0] = "A. "
	if err := validateMockExamSection(section, mockExamSpecs[0]); err == nil {
		t.Fatal("paraphrase relaxation must not allow missing option context")
	}
}

func TestMockExamRestoresOnlyUniqueContinuousSourceQuotes(t *testing.T) {
	source := "The workshop uses oak; the entrance requires slate. Annual attendance reached 1,000 visitors in June."
	quote := "The workshop uses oak, the entrance requires slate."
	restored := mockExamRestoreSourceQuote(source, quote)
	if restored == quote || !mockExamExactEvidence(source, restored) {
		t.Fatal("punctuation-only quote was not restored to exact source text")
	}
	for _, invalid := range []string{
		"The workshop uses copper, the entrance requires slate.",
		"The workshop uses oak, the entrance never requires slate.",
		"The workshop uses oak, entrance requires slate.",
		"Annual attendance reached 1.000 visitors in June.",
		"The workshop uses oak... the entrance requires slate.",
	} {
		if mockExamRestoreSourceQuote(source, invalid) != invalid {
			t.Fatal("invalid quote was repaired by changing or omitting source information")
		}
	}
	if got := mockExamRestoreSourceQuote(source+" "+source, quote); got != quote {
		t.Fatal("ambiguous quote should not be restored")
	}
	section := mockExamFixtureSection(0, 7)
	section.Questions[0].Evidence = strings.ReplaceAll(section.Questions[0].Evidence, ",", ";")
	if err := validateMockExamSection(section, mockExamSpecs[0]); err == nil {
		t.Fatal("trust boundary must still reject non-verbatim stored evidence")
	}
}

func TestMockExamEvidencePreservesNumericMeaning(t *testing.T) {
	for _, pair := range [][2]string{{"-15", "+15"}, {"-15", "15"}, {"−15", "15"}, {"1,000", "1.000"}, {"15%", "15"}, {"£15", "$15"}} {
		source := "The recorded balance was " + pair[0] + " after the final adjustment."
		wrong := "The recorded balance was " + pair[1] + " after the final adjustment."
		if mockExamRestoreSourceQuote(source, wrong) != wrong {
			t.Errorf("quote restoration changed numeric meaning: %s -> %s", pair[1], pair[0])
		}
		if mockExamContainsPhrase(source, pair[1]) {
			t.Errorf("evidence incorrectly supports %s instead of %s", pair[1], pair[0])
		}
		if !mockExamContainsPhrase(source, pair[0]) {
			t.Errorf("correct numeric phrase rejected: %s", pair[0])
		}
	}
	q := domain.QuizQuestion{Type: "sentence_completion", Question: "The account balance was ____. Write NO MORE THAN TWO WORDS.", AnswerKey: "+15 pounds", Evidence: "The account balance fell to -15 pounds after payment."}
	if validateMockExamQuestion(q, q.Evidence, "READING") == nil {
		t.Fatal("wrong signed completion was accepted")
	}
	if mockExamExactEvidence("The balance was -15 pounds after payment.", "15 pounds after payment.") {
		t.Fatal("a quote starting inside a signed number must not be accepted")
	}
}

func TestMockExamDraftDiagnosticsCollectMultipleErrors(t *testing.T) {
	s := mockExamFixtureSection(0, 7)
	s.Questions[1].Question = "Write NO MORE THAN THREE WORDS."
	s.Questions[2].AnswerKey = "unrelated astronomy"
	err := mockExamDraftDiagnostics(s, mockExamSpecs[0])
	if err == nil || !strings.Contains(err.Error(), "question 2:") || !strings.Contains(err.Error(), "question 3:") {
		t.Fatal("regeneration did not receive all question errors")
	}
}

func TestMockExamAuditPrivateDrafts(t *testing.T) {
	directory := os.Getenv("IELTS_MOCK_AUDIT_DIR")
	if directory == "" {
		t.Skip("explicit private offline audit only")
	}
	files, err := filepath.Glob(filepath.Join(directory, "private-target-*.json"))
	if err != nil {
		t.Fatal("cannot enumerate private drafts")
	}
	namePattern := regexp.MustCompile(`private-target-([678])-(LISTENING_[1-4]|READING_[1-3]|WRITING)-(?:attempt|generation)-\d+\.json$`)
	for _, file := range files {
		match := namePattern.FindStringSubmatch(filepath.Base(file))
		if len(match) != 3 {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal("cannot read private draft")
		}
		var d mockExamDraft
		if json.Unmarshal(data, &d) != nil {
			t.Fatal("cannot decode private draft")
		}
		for _, spec := range mockExamSpecs {
			if spec.key != match[2] {
				continue
			}
			s := domain.MockExamSection{Key: spec.key, Skill: spec.skill, DurationSeconds: spec.seconds, Title: d.Title, Instructions: d.Instructions, Passage: d.Passage, AudioScript: d.AudioScript, Questions: d.Questions, WritingPrompts: d.WritingPrompts}
			restored := 0
			for i := range s.Questions {
				quote := mockExamRestoreSourceQuote(s.Passage+s.AudioScript, s.Questions[i].Evidence)
				if quote != s.Questions[i].Evidence {
					restored++
				}
				s.Questions[i].Evidence = quote
			}
			err := mockExamDraftDiagnostics(s, spec)
			t.Logf("file=%s restored_quotes=%d deterministic_gate=%v (not an AI approval)", filepath.Base(file), restored, err)
		}
	}
}

func TestMockExamBlindPromptHidesAllAnswerFields(t *testing.T) {
	section := mockExamFixtureSection(4, 7)
	q := &section.Questions[0]
	q.AnswerKey = "SECRET_AUTHOR_KEY"
	q.Evidence = "SECRET_AUTHOR_EVIDENCE"
	q.ParagraphRef = "SECRET_PARAGRAPH"
	q.Answers = []string{"SECRET_ANSWERS"}
	q.Statements = []domain.ReadingStatement{{Answer: "SECRET_STATEMENT"}}
	section.Answers = []string{"SECRET_USER_ANSWER"}
	section.Responses = []string{"SECRET_USER_RESPONSE"}
	prompt := mockExamBlindReviewPrompt(section)
	if !strings.Contains(prompt, "section-local internal identifier") {
		t.Fatal("review must distinguish internal IDs from candidate numbering")
	}
	for _, secret := range []string{"SECRET_", "answerKey", "paragraphRef", "QualityApproved"} {
		if strings.Contains(prompt, secret) {
			t.Fatalf("blind review leaked field %s", secret)
		}
	}
	if !strings.Contains(prompt, section.Passage) { // JSON escapes newlines, so decode for the actual source comparison.
		var data struct {
			Passage string `json:"passage"`
		}
		if err := json.Unmarshal([]byte(strings.SplitN(prompt, "SECTION_JSON:\n", 2)[1]), &data); err != nil || data.Passage != section.Passage {
			t.Fatal("blind review lost the source")
		}
	}
}

// 离线重放真实整卷及其最后一次盲审；不调用模型，也不生成替代题目。
func TestMockExamAuditPrivatePaper(t *testing.T) {
	path := os.Getenv("IELTS_MOCK_AUDIT_PAPER")
	if path == "" {
		t.Skip("explicit private full-paper audit only")
	}
	if !filepath.IsAbs(path) {
		t.Fatal("private paper path must be absolute")
	}
	data, err := os.ReadFile(path)
	var paper domain.MockExam
	if err != nil || json.Unmarshal(data, &paper) != nil {
		t.Fatal("cannot read private full paper")
	}
	if err = ValidateMockExamPaper(paper); err != nil {
		t.Fatal(err)
	}
	readingWords, listeningWords, readingQuestions, listeningQuestions := 0, 0, 0, 0
	for _, section := range paper.Sections {
		pattern := fmt.Sprintf("private-target-%.0f-%s-review-*.json", section.TargetBand, section.Key)
		files, globErr := filepath.Glob(filepath.Join(filepath.Dir(path), pattern))
		if globErr != nil || len(files) == 0 {
			t.Fatalf("section %s missing private blind-review evidence", section.Key)
		}
		var review mockExamReview
		data, err = os.ReadFile(files[len(files)-1]) // Glob 按文件名排序；编号为两位。
		if err != nil || json.Unmarshal(data, &review) != nil {
			t.Fatalf("section %s unreadable blind review", section.Key)
		}
		for i := range review.Answers {
			review.Answers[i].Evidence = mockExamRestoreSourceQuote(section.Passage+section.AudioScript, review.Answers[i].Evidence)
		}
		if err = validateMockExamReview(section, review); err != nil {
			t.Fatalf("section %s persisted paper/review mismatch: %v", section.Key, err)
		}
		words := mockExamWordCount(section.Passage + section.AudioScript)
		switch section.Skill {
		case "READING":
			readingWords += words
			readingQuestions += len(section.Questions)
		case "LISTENING":
			listeningWords += words
			listeningQuestions += len(section.Questions)
		}
		t.Logf("section=%s words=%d questions=%d writing_tasks=%d blind_review_matches=true", section.Key, words, len(section.Questions), len(section.WritingPrompts))
	}
	t.Logf("target=%.1f reading_words=%d reading_questions=%d listening_words=%d listening_questions=%d (automated checks, not examiner calibration)", paper.Sections[0].TargetBand, readingWords, readingQuestions, listeningWords, listeningQuestions)
}

type mockExamProvider struct {
	sourceCalls              map[string]int
	modifySource             func(string, int, *mockExamSource)
	mu                       sync.Mutex
	generationCalls          map[string]int
	reviewCalls              map[string]int
	regenerationFeedback     bool
	modifyDraft              func(string, int, *mockExamDraft)
	modifyReview             func(string, int, *mockExamReview)
	malformedGenerationFirst map[string]bool
	status                   int
	active                   atomic.Int32
	peak                     atomic.Int32
}

func (p *mockExamProvider) serve(w http.ResponseWriter, r *http.Request) {
	active := p.active.Add(1)
	defer p.active.Add(-1)
	for old := p.peak.Load(); active > old; old = p.peak.Load() {
		if p.peak.CompareAndSwap(old, active) {
			break
		}
	}
	if p.status != 0 {
		w.WriteHeader(p.status)
		_, _ = w.Write([]byte("provider echo SECRET_DO_NOT_LOG"))
		return
	}
	var req struct {
		Model    string                           `json:"model"`
		Messages []struct{ Role, Content string } `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) != 2 {
		http.Error(w, "bad request", 400)
		return
	}
	prompt := req.Messages[1].Content
	sourceRequest := strings.Contains(prompt, "practice paper. SOURCE_ONLY stage.")
	reviewRequest := strings.Contains(prompt, "SECTION_JSON:\n")
	key := ""
	if reviewRequest {
		var section struct {
			Key string `json:"key"`
		}
		_ = json.Unmarshal([]byte(strings.SplitN(prompt, "SECTION_JSON:\n", 2)[1]), &section)
		key = section.Key
	} else {
		match := regexp.MustCompile(`Create section (\w+) of`).FindStringSubmatch(prompt)
		if len(match) == 2 {
			key = match[1]
		}
	}
	index := -1
	for i, spec := range mockExamSpecs {
		if spec.key == key {
			index = i
		}
	}
	if index < 0 {
		http.Error(w, "unknown section", 400)
		return
	}
	section := mockExamFixtureSection(index, 7)
	p.mu.Lock()
	var result any
	if sourceRequest {
		p.sourceCalls[key]++
		source := mockExamSource{Title: section.Title, Passage: section.Passage, AudioScript: section.AudioScript}
		if p.modifySource != nil {
			p.modifySource(key, p.sourceCalls[key], &source)
		}
		result = source
	} else if reviewRequest {
		p.reviewCalls[key]++
		review := mockExamFixtureReview(section)
		if p.modifyReview != nil {
			p.modifyReview(key, p.reviewCalls[key], &review)
		}
		result = review
	} else {
		p.generationCalls[key]++
		if strings.Contains(prompt, "Previous attempt failed.") {
			p.regenerationFeedback = true
		}
		if p.malformedGenerationFirst != nil && p.malformedGenerationFirst[key] && p.generationCalls[key] == 1 {
			result = "{malformed"
		} else {
			draft := mockExamDraft{Title: section.Title, Instructions: section.Instructions, Passage: section.Passage, AudioScript: section.AudioScript, Questions: section.Questions, WritingPrompts: section.WritingPrompts}
			if p.modifyDraft != nil {
				p.modifyDraft(key, p.generationCalls[key], &draft)
			}
			result = draft
		}
	}
	p.mu.Unlock()
	content, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": string(content)}}}})
}

func mockExamTestGenerator(t *testing.T, p *mockExamProvider) *OpenAIGenerator {
	t.Helper()
	p.generationCalls = map[string]int{}
	p.sourceCalls = map[string]int{}
	p.reviewCalls = map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(server.Close)
	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	g.Client = server.Client()
	return g
}

func TestMockExamProviderSuccess(t *testing.T) {
	p := &mockExamProvider{}
	g := mockExamTestGenerator(t, p)
	paper, err := g.GenerateMockExam(context.Background(), 6.5)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateMockExamPaper(paper); err != nil {
		t.Fatal(err)
	}
	for i, s := range paper.Sections {
		if s.Key != mockExamSpecs[i].key || s.TargetBand != 6.5 || !s.QualityApproved {
			t.Fatal("metadata/order mismatch")
		}
		if p.generationCalls[s.Key] != 1 || p.reviewCalls[s.Key] != 1 {
			t.Fatal("expected one generation and one review per section")
		}
	}
	if p.peak.Load() > 2 {
		t.Fatal("concurrency limit exceeded")
	}
	encoded, err := json.Marshal(paper)
	if err != nil {
		t.Fatal(err)
	}
	var decoded domain.MockExam
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Sections[0].AudioScript == "" {
		t.Fatal("persistent audio script was lost")
	}
}

func TestMockExamProviderRegeneratesOnReviewFailure(t *testing.T) {
	p := &mockExamProvider{modifyReview: func(key string, attempt int, r *mockExamReview) {
		if key == "LISTENING_1" && attempt == 1 {
			r.Answers[0].Answer = "B"
			r.Answers[0].Feedback = "The competing option remains plausible because the chronology is unclear."
		}
	}}
	g := mockExamTestGenerator(t, p)
	if _, err := g.GenerateMockExam(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if p.generationCalls["LISTENING_1"] != 2 || p.reviewCalls["LISTENING_1"] != 2 || !p.regenerationFeedback {
		t.Fatal("review failure did not drive full regeneration")
	}
	if p.sourceCalls["LISTENING_1"] != 1 {
		t.Fatal("question repair regenerated the frozen source")
	}
}

func TestMockExamProviderRetriesMalformedQuestionJSONWithinFormatBudget(t *testing.T) {
	p := &mockExamProvider{malformedGenerationFirst: map[string]bool{"LISTENING_1": true}}
	g := mockExamTestGenerator(t, p)
	section, err := g.generateMockExamSection(context.Background(), mockExamSpecs[0], 7)
	if err != nil || !section.QualityApproved {
		t.Fatalf("malformed first response should recover: section=%+v err=%v", section, err)
	}
	if p.sourceCalls["LISTENING_1"] != 1 || p.generationCalls["LISTENING_1"] != 2 || p.reviewCalls["LISTENING_1"] != 1 {
		t.Fatalf("unexpected retry counts: source=%d generation=%d review=%d", p.sourceCalls["LISTENING_1"], p.generationCalls["LISTENING_1"], p.reviewCalls["LISTENING_1"])
	}
}

func TestMockExamReviewConformingItemsKeepsOnlyIndividuallyVerifiedQuestions(t *testing.T) {
	section := mockExamFixtureSection(0, 7)
	review := mockExamFixtureReview(section)
	review.Answers[8].Answer = "B"

	fixed := mockExamReviewConformingItems(section, review)
	if len(fixed) != len(section.Questions)-1 {
		t.Fatalf("got %d fixed questions, want %d", len(fixed), len(section.Questions)-1)
	}
	if _, ok := fixed[8]; ok {
		t.Fatal("question with a blind-answer disagreement was frozen")
	}

	review = mockExamFixtureReview(section)
	review.FormCorrect = false
	if fixed = mockExamReviewConformingItems(section, review); len(fixed) != 0 {
		t.Fatal("questions from a globally rejected review were frozen")
	}

	review = mockExamFixtureReview(section)
	review.Approved = false
	review.Answers[8].Unique = false
	if fixed = mockExamReviewConformingItems(section, review); len(fixed) != len(section.Questions)-1 {
		t.Fatalf("single blind-review failure discarded other verified questions: got %d", len(fixed))
	}
	if _, ok := fixed[8]; ok {
		t.Fatal("blind-review failure was frozen")
	}
}

func TestMockExamReviewQuoteCorrection(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(fmt.Sprintf("persistent_%v", persistent), func(t *testing.T) {
			p := &mockExamProvider{modifyReview: func(_ string, attempt int, r *mockExamReview) {
				if persistent || attempt == 1 {
					r.Answers[0].Evidence = "A quotation that is absent from the source."
				}
			}}
			g := mockExamTestGenerator(t, p)
			section, err := g.generateMockExamSection(context.Background(), mockExamSpecs[0], 7)
			if persistent {
				if err == nil || section.QualityApproved || p.reviewCalls["LISTENING_1"] != 6 || p.generationCalls["LISTENING_1"] != 3 {
					t.Fatal("persistent invalid quotes must fail within bounded review/question attempts")
				}
			} else if err != nil || !section.QualityApproved || p.sourceCalls["LISTENING_1"] != 1 || p.generationCalls["LISTENING_1"] != 1 || p.reviewCalls["LISTENING_1"] != 2 {
				t.Fatalf("quote-only correction should not regenerate source or questions: %v", err)
			}
		})
	}
	section := mockExamFixtureSection(0, 7)
	for _, mutate := range []func(*mockExamReview){
		func(r *mockExamReview) { r.Approved = false },
		func(r *mockExamReview) { r.MixedDifficulty = false },
		func(r *mockExamReview) { r.Answers[1].Unique = false },
		func(r *mockExamReview) { r.Answers[1].Supported = false },
		func(r *mockExamReview) { r.Answers[1].Answer = "different" },
		func(r *mockExamReview) { r.Answers[1].Feedback = "" },
		func(r *mockExamReview) { r.Answers[1].Number = 1 },
	} {
		r := mockExamFixtureReview(section)
		r.Answers[0].Evidence = "Invalid quote appearing before a more serious semantic problem."
		mutate(&r)
		if mockExamReviewQuoteRepairable(section, r) {
			t.Fatal("quote repair must not hide any semantic rejection or answer disagreement")
		}
	}
}

func TestMockExamFormatRepairsDoNotConsumeQualityBudget(t *testing.T) {
	p := &mockExamProvider{
		modifyDraft: func(_ string, attempt int, d *mockExamDraft) {
			if attempt <= 2 {
				d.Questions = nil
			}
		},
		modifyReview: func(_ string, attempt int, r *mockExamReview) {
			if attempt <= 2 {
				r.Approved, r.MixedDifficulty = false, false
			}
		},
	}
	g := mockExamTestGenerator(t, p)
	s, err := g.generateMockExamSection(context.Background(), mockExamSpecs[0], 7)
	if err != nil || !s.QualityApproved || p.sourceCalls["LISTENING_1"] != 1 || p.generationCalls["LISTENING_1"] != 5 || p.reviewCalls["LISTENING_1"] != 3 {
		t.Fatalf("independent bounded formatting and quality corrections failed: %v", err)
	}
}

func TestMockExamSourceLengthRepairIsSeparateAndBounded(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(fmt.Sprintf("persistent_%v", persistent), func(t *testing.T) {
			p := &mockExamProvider{modifySource: func(key string, attempt int, source *mockExamSource) {
				if key == "LISTENING_1" && (persistent || attempt == 1) {
					source.AudioScript += strings.Repeat(" extra", 180)
				}
			}}
			g := mockExamTestGenerator(t, p)
			section, err := g.generateMockExamSection(context.Background(), mockExamSpecs[0], 7)
			if persistent {
				if err == nil || section.QualityApproved || p.sourceCalls["LISTENING_1"] != 3 || p.generationCalls["LISTENING_1"] != 0 || p.reviewCalls["LISTENING_1"] != 0 {
					t.Fatal("invalid source escaped bounded source-only gate")
				}
			} else if err != nil || !section.QualityApproved || p.sourceCalls["LISTENING_1"] != 2 || p.generationCalls["LISTENING_1"] != 1 || p.reviewCalls["LISTENING_1"] != 1 {
				t.Fatalf("source correction did not precede a single question/review pass: %v", err)
			}
		})
	}
}

func TestMockExamListeningSourcePromptsKeepWordCountSafetyMargin(t *testing.T) {
	prompt := mockExamSourcePrompt(mockExamSpecs[2], 7.5)
	for _, required := range []string{"690-730 spoken words TOTAL", "program count excludes speaker labels", "21-23 turns", "ACADEMIC DISCUSSION ARCHITECTURE", "separate decision or interpretation chains", "at least 4 separate decision or interpretation chains"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("listening source prompt lost word-count guidance %q", required)
		}
	}
	lecture := mockExamSourcePrompt(mockExamSpecs[3], 6.5)
	for _, required := range []string{"ACADEMIC LECTURE ARCHITECTURE", "separate interpretation or method-evaluation chains", "at least 3 separate interpretation or method-evaluation chains", "Never put both decisive facts and the complete answer in one sentence"} {
		if !strings.Contains(lecture, required) {
			t.Fatalf("academic lecture source prompt lost architecture guidance %q", required)
		}
	}
	revision := mockExamSourceRevisionGuidance(mockExamSpecs[2])
	for _, required := range []string{"COMPLETE source", "700-760 spoken English words", "absolute drafting ceiling of 800", "ten distinct questions"} {
		if !strings.Contains(revision, required) {
			t.Fatalf("listening source revision lost safety guidance %q", required)
		}
	}
}

func TestMockExamProviderRegeneratesBeforeReviewOnGateFailure(t *testing.T) {
	p := &mockExamProvider{modifyDraft: func(key string, attempt int, d *mockExamDraft) {
		if key == "LISTENING_1" && attempt == 1 {
			d.AudioScript = "short script"
		}
	}}
	g := mockExamTestGenerator(t, p)
	if _, err := g.GenerateMockExam(context.Background(), 8); err != nil {
		t.Fatal(err)
	}
	if p.generationCalls["LISTENING_1"] != 2 || p.reviewCalls["LISTENING_1"] != 1 {
		t.Fatal("invalid draft should never enter blind review")
	}
}

func TestMockExamProviderFailureBounded(t *testing.T) {
	for _, failure := range []string{"review", "gate"} {
		t.Run(failure, func(t *testing.T) {
			p := &mockExamProvider{}
			if failure == "review" {
				p.modifyReview = func(key string, _ int, r *mockExamReview) {
					if key == "LISTENING_1" {
						r.Answers[0].Unique = false
					}
				}
			} else {
				p.modifyDraft = func(key string, _ int, d *mockExamDraft) {
					if key == "LISTENING_1" {
						d.Questions = nil
					}
				}
			}
			g := mockExamTestGenerator(t, p)
			paper, err := g.GenerateMockExam(context.Background(), 7)
			if err == nil || len(paper.Sections) != 0 {
				t.Fatal("failed generation returned a paper")
			}
			if p.generationCalls["LISTENING_1"] != 3 {
				t.Fatalf("attempts=%d, want initial + 2 regenerations", p.generationCalls["LISTENING_1"])
			}
		})
	}
}

func TestMockExamProviderTransportFailureIsSanitized(t *testing.T) {
	p := &mockExamProvider{status: 401}
	g := mockExamTestGenerator(t, p)
	_, err := g.GenerateMockExam(context.Background(), 7)
	if err == nil || strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), "model request failed") {
		t.Fatal("expected sanitized provider failure")
	}
}

func TestMockExamInputAndCancellation(t *testing.T) {
	p := &mockExamProvider{}
	g := mockExamTestGenerator(t, p)
	for _, band := range []float64{0, 5.5, 6.1, 8.5, math.NaN(), math.Inf(1)} {
		if _, err := g.GenerateMockExam(context.Background(), band); err == nil {
			t.Fatal("invalid target accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.GenerateMockExam(ctx, 7); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation not propagated")
	}
	g.UpdateModelConfig(domain.ModelConfig{APIKey: "", Model: "test-model", BaseURL: "http://127.0.0.1"})
	if _, err := g.GenerateMockExam(context.Background(), 7); err == nil {
		t.Fatal("empty API key accepted")
	}
	if len(p.generationCalls) != 0 {
		t.Fatal("invalid input should not call the provider")
	}
}

func TestMockExamGlobalRequestLimitAndQueuedCancellation(t *testing.T) {
	// 两个不同实例占满全局请求槽；第三个实例必须可取消地等待。
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": "{}"}}}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := NewOpenAIGenerator("test", "model", server.URL)
			_, _ = g.mockExamCompletion(ctx, "system", "prompt", "MOCK_EXAM_REVIEW")
		}()
	}
	defer func() { close(release); wg.Wait() }()
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("requests did not start")
		}
	}
	queued, cancelQueued := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancelQueued()
	g := NewOpenAIGenerator("test", "model", server.URL)
	if _, err := g.mockExamCompletion(queued, "system", "prompt", "MOCK_EXAM_REVIEW"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("queued cancellation lost")
	}
	if peak.Load() != 2 {
		t.Fatal("global process request limit not enforced")
	}
}

func TestMockExamInFlightCancellation(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	g := NewOpenAIGenerator("test", "model", server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := g.GenerateMockExam(ctx, 7); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("generation did not cancel promptly")
	}
}

func TestMockExamStrictJSON(t *testing.T) {
	for _, content := range []string{"", "null", "{", `{"title":"x","qualityApproved":true}`, `{} {}`, strings.Repeat("x", 128*1024+1)} {
		var draft mockExamDraft
		err := decodeMockExamJSON(content, &draft)
		if err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}

// Opt-in only. PowerShell example (credentials must already be in the process
// environment, never paste them into commands/logs):
// $env:IELTS_MOCK_LIVE='1'; go test ./internal/ai -run '^TestLiveMockExam$' -count=1 -timeout=65m
// Required: OPENAI_API_KEY, OPENAI_MODEL, OPENAI_BASE_URL.
// Optional: IELTS_MOCK_LIVE_OUTPUT_DIR, an existing private directory. Each run
// creates a new subdirectory; only the key-free learner paper and counts are saved.
// IELTS_MOCK_LIVE_PRIVATE=1 additionally retains generated drafts and full papers
// in that explicitly configured directory, for private content/audio audits only.
func TestLiveMockExam(t *testing.T) {
	if os.Getenv("IELTS_MOCK_LIVE") != "1" {
		t.Skip("set IELTS_MOCK_LIVE=1 to run paid configured-provider validation")
	}
	// go test runs in internal/ai. Load without overriding configured process env.
	// Never include loader errors: they may contain malformed secret-bearing lines.
	if err := godotenv.Load(filepath.Join("..", "..", ".env")); err != nil {
		if os.Getenv("OPENAI_API_KEY") == "" {
			t.Fatal("cannot load server .env and no API key is configured")
		}
	}
	serverRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("cannot resolve server root")
	}
	modelConfig, configSource, err := mockExamLiveModelConfig(serverRoot, config.Load())
	if err != nil {
		t.Fatal(err)
	}
	if modelConfig.APIKey == "" {
		t.Fatal("live validation requires a configured model API key; value is never printed")
	}
	g := NewOpenAIGenerator(modelConfig.APIKey, modelConfig.Model, modelConfig.BaseURL)
	g.UpdateModelConfig(modelConfig)
	t.Logf("live model=%s configuration_source=%s", g.modelName(), configSource)
	output := ""
	private := os.Getenv("IELTS_MOCK_LIVE_PRIVATE") == "1"
	if private && os.Getenv("IELTS_MOCK_LIVE_OUTPUT_DIR") == "" {
		t.Fatal("private live capture requires an explicit private output directory")
	}
	if dir := os.Getenv("IELTS_MOCK_LIVE_OUTPUT_DIR"); dir != "" {
		var err error
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal("cannot create private live output directory")
		}
		output, err = os.MkdirTemp(dir, "ielts-academic-v2-")
		if err != nil {
			t.Fatal("cannot create private live report directory")
		}
		t.Logf("private artifacts: %s", output)
	}
	for _, band := range []float64{6, 7, 8} {
		t.Run(fmt.Sprintf("target_%.0f", band), func(t *testing.T) {
			var capture *mockExamPrivateCapture
			if private {
				capture = &mockExamPrivateCapture{base: http.DefaultTransport, directory: output, band: band, attempts: map[string]int{}}
				g.Client.Transport = capture
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			started := time.Now()
			paper, err := g.GenerateMockExam(ctx, band)
			if capture != nil && capture.failed {
				t.Error("one or more private generated draft artifacts could not be saved")
			}
			if err != nil {
				if output != "" {
					report, marshalErr := json.MarshalIndent(struct {
						Model          string  `json:"model"`
						ConfigSource   string  `json:"configurationSource"`
						TargetBand     float64 `json:"targetBand"`
						Approved       bool    `json:"approved"`
						Reason         string  `json:"reason"`
						ElapsedSeconds float64 `json:"elapsedSeconds"`
					}{g.modelName(), configSource, band, false, err.Error(), time.Since(started).Seconds()}, "", "  ")
					if marshalErr != nil || os.WriteFile(filepath.Join(output, fmt.Sprintf("target-%.0f-failure.json", band)), report, 0600) != nil {
						t.Error("cannot save safe failure report")
					}
				}
				t.Fatalf("target %.1f generation failed: %v", band, err)
			}
			if err = ValidateMockExamPaper(paper); err != nil {
				t.Fatalf("target %.1f validation failed: %v", band, err)
			}
			if private {
				data, encodeErr := json.MarshalIndent(paper, "", "  ")
				if encodeErr != nil || os.WriteFile(filepath.Join(output, fmt.Sprintf("private-paper-target-%.0f.json", band)), data, 0600) != nil {
					t.Fatal("cannot save private generated paper")
				}
			}
			report := struct {
				Model               string `json:"model"`
				ConfigurationSource string `json:"configurationSource"`
				Paper               any    `json:"paper"`
			}{g.modelName(), configSource, mockExamSafeReport(paper, time.Since(started))}
			if output != "" {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					t.Fatal("cannot encode safe live report")
				}
				if err = os.WriteFile(filepath.Join(output, fmt.Sprintf("target-%.0f.json", band)), data, 0600); err != nil {
					t.Fatal("cannot save safe live report")
				}
			}
			t.Logf("target %.1f: eight reviewed sections, 40 listening + 40 reading questions, two writing tasks; elapsed %s", band, time.Since(started).Round(time.Second))
		})
	}
}

// Resolve paths relative to apps/server as the application does, not the Go
// test package working directory. Open existing SQLite read-only; no migration.
func mockExamLiveModelConfig(serverRoot string, cfg config.Config) (domain.ModelConfig, string, error) {
	model := domain.ModelConfig{Provider: "OPENAI", Model: cfg.OpenAIModel, BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIAPIKey}
	if cfg.SQLitePath == "" {
		return model, "environment", nil
	}
	path := cfg.SQLitePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(serverRoot, path)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return model, "environment", nil
	} else if err != nil {
		return domain.ModelConfig{}, "", errors.New("cannot inspect configured SQLite file")
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	dsn := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return domain.ModelConfig{}, "", errors.New("cannot open configured SQLite read-only")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='model_configs'`).Scan(&exists); err != nil {
		return domain.ModelConfig{}, "", errors.New("cannot inspect persisted model configuration")
	}
	if exists == 0 {
		return model, "environment", nil
	}
	persisted, err := store.ReadSQLiteModelConfig(ctx, db)
	if errors.Is(err, sql.ErrNoRows) {
		return model, "environment", nil
	}
	if err != nil {
		return domain.ModelConfig{}, "", errors.New("cannot read persisted model configuration")
	}
	return persisted, "sqlite", nil
}

func TestMockExamLiveConfigUsesPersistedModel(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "models.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE model_configs (id INTEGER PRIMARY KEY,provider TEXT,model TEXT,base_url TEXT,api_key TEXT,updated_at TEXT); INSERT INTO model_configs VALUES (1,'OPENAI_COMPATIBLE','persisted-model','http://127.0.0.1','private-test-key','2026-09-17 09:51:51')`); err != nil {
		t.Fatal(err)
	}
	got, source, err := mockExamLiveModelConfig(directory, config.Config{SQLitePath: "models.sqlite", OpenAIModel: "environment-model", OpenAIAPIKey: "environment-key"})
	if err != nil {
		t.Fatal(err)
	}
	if source != "sqlite" || got.Model != "persisted-model" || got.Provider != "OPENAI_COMPATIBLE" || got.APIKey != "private-test-key" {
		t.Fatal("live harness did not select the persisted model configuration")
	}
}

// Test-only transport observer: never captures request headers, URLs, config,
// provider errors, or raw response envelopes. It saves generated content DTOs;
// source-review evidence also includes only the reviewed source and its hash.
// Production has no capture hook.
type mockExamPrivateCapture struct {
	base       http.RoundTripper
	directory  string
	band       float64
	sectionKey string
	mu         sync.Mutex
	attempts   map[string]int
	failed     bool
}

func (capture *mockExamPrivateCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := capture.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "private_capture section=%s transport_error_type=%T context_cancelled=%t\n", capture.sectionKey, err, req.Context().Err() != nil)
	} else if response.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "private_capture section=%s http_status=%d\n", capture.sectionKey, response.StatusCode)
	}
	if err != nil || response.StatusCode != http.StatusOK || req.GetBody == nil {
		return response, err
	}
	requestBody, err := req.GetBody()
	if err != nil {
		return response, nil
	}
	var request struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
		Input []struct {
			Content string `json:"content"`
		} `json:"input"`
	}
	decodeErr := json.NewDecoder(requestBody).Decode(&request)
	_ = requestBody.Close()
	if decodeErr != nil {
		return response, nil
	}
	messages := request.Messages
	if len(messages) == 0 {
		messages = request.Input
	}
	if len(messages) != 2 {
		return response, nil
	}
	userContent := messages[1].Content
	match := regexp.MustCompile(`^Create section (LISTENING_[1-4]|READING_[1-3]|WRITING) of`).FindStringSubmatch(userContent)
	kind, key := "generation", ""
	var reviewedSource *mockExamSource
	if len(match) == 2 {
		key = match[1]
		if strings.Contains(userContent, "practice paper. SOURCE_ONLY stage.") {
			kind = "source"
		}
	} else if parts := strings.SplitN(userContent, "SECTION_JSON:\n", 2); len(parts) == 2 {
		var section struct {
			Key string `json:"key"`
		}
		if json.Unmarshal([]byte(parts[1]), &section) == nil {
			key = section.Key
			kind = "review"
		}
	} else if strings.Contains(userContent, "SPEAKING_FEEDBACK_REVIEW:") {
		key, kind = "SPEAKING", "speaking-review"
	} else if strings.Contains(userContent, "<CANDIDATE_TURNS>") {
		key, kind = "SPEAKING", "speaking-evaluation"
	} else if strings.Contains(userContent, "LISTENING_SOURCE_REVIEW:") {
		key, kind = capture.sectionKey, "source-review"
		parts := strings.SplitN(userContent, "LISTENING_SOURCE_REVIEW:\n", 2)
		if len(parts) == 2 {
			var source mockExamSource
			if json.Unmarshal([]byte(parts[1]), &source) == nil {
				reviewedSource = &source
			}
		}
	} else if strings.Contains(userContent, "Untrusted diagnostic data:") && strings.Contains(userContent, "Every dimension marked FAIL") {
		key, kind = capture.sectionKey, "source-revision"
	} else if strings.Contains(userContent, "LISTENING_DIFFICULTY_SOURCE:") {
		key, kind = capture.sectionKey, "difficulty-review"
	} else if strings.Contains(userContent, "DIFFICULTY_REPAIR_DATA:") || strings.Contains(userContent, "LISTENING_FAILED_ITEM_REPAIR:") {
		key, kind = capture.sectionKey, "question-repair"
	} else if strings.Contains(userContent, "LISTENING_PLAN_VIABILITY:") {
		key, kind = capture.sectionKey, "plan-viability"
	} else if strings.Contains(userContent, "LISTENING_PLAN_CONFORMANCE:") {
		key, kind = capture.sectionKey, "plan-conformance"
	} else if strings.Contains(userContent, "Design a binding construction plan") && strings.Contains(userContent, "FROZEN_SOURCE_JSON:") {
		key, kind = capture.sectionKey, "question-plan"
	}
	if !regexp.MustCompile(`^(LISTENING_[1-4]|READING_[1-3]|WRITING|SPEAKING)$`).MatchString(key) {
		return response, nil
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	content, err := extractModelTextFromResponse(body)
	if err != nil {
		var envelope struct {
			Status            string `json:"status"`
			IncompleteDetails *struct {
				Reason string `json:"reason"`
			} `json:"incomplete_details"`
			Error *struct {
				Type string `json:"type"`
				Code string `json:"code"`
			} `json:"error"`
			Output []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"output"`
		}
		_ = json.Unmarshal(body, &envelope)
		_, completionTokens, totalTokens, usageReported := extractModelUsageFromResponse(body)
		outputTypes := make([]string, 0, len(envelope.Output))
		for _, item := range envelope.Output {
			outputTypes = append(outputTypes, strings.TrimSpace(item.Type)+":"+strings.TrimSpace(item.Status))
		}
		incompleteReason, errorType, errorCode := "", "", ""
		if envelope.IncompleteDetails != nil {
			incompleteReason = envelope.IncompleteDetails.Reason
		}
		if envelope.Error != nil {
			errorType, errorCode = envelope.Error.Type, envelope.Error.Code
		}
		document := map[string]any{
			"responseStatus": envelope.Status, "incompleteReason": incompleteReason,
			"errorType": errorType, "errorCode": errorCode, "outputTypes": outputTypes,
			"completionTokens": completionTokens, "totalTokens": totalTokens, "usageReported": usageReported,
		}
		data, encodeErr := json.MarshalIndent(document, "", "  ")
		if encodeErr == nil {
			capture.mu.Lock()
			identifier := key + "-" + kind + "-no-text"
			capture.attempts[identifier]++
			name := fmt.Sprintf("private-target-%.0f-%s-%s-%02d.json", capture.band, key, kind+"-no-text", capture.attempts[identifier])
			if os.WriteFile(filepath.Join(capture.directory, name), data, 0600) != nil {
				capture.failed = true
			}
			capture.mu.Unlock()
		}
		return response, nil
	}
	var document any
	if kind == "source" {
		document = &mockExamSource{}
	} else if kind == "review" {
		document = &mockExamReview{}
	} else if kind == "speaking-evaluation" {
		document = &domain.SpeakingEvaluation{}
	} else if kind == "difficulty-review" {
		document = &domain.ListeningDifficultyReview{}
	} else if kind == "question-plan" {
		document = &listeningQuestionPlan{}
	} else if kind == "question-repair" {
		document = &listeningQuestionRepair{}
	} else if kind == "plan-conformance" || kind == "plan-viability" {
		document = &listeningPlanConformanceReview{}
	} else if kind == "source-review" {
		document = &listeningSourceReview{}
	} else if kind == "source-revision" {
		document = &listeningSourceRevision{}
	} else if kind == "speaking-review" {
		document = &struct {
			Approved *bool  `json:"approved"`
			Feedback string `json:"feedback"`
		}{}
	} else {
		document = &mockExamDraft{}
	}
	if parseErr := json.Unmarshal([]byte(sanitizeJSONLikeContent(content)), document); parseErr != nil {
		// 格式异常也保留诊断，但不保存原始响应、请求或认证信息。
		var envelope struct {
			Choices []struct {
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		_ = json.Unmarshal(body, &envelope)
		finishReason := "unknown"
		if len(envelope.Choices) > 0 {
			switch envelope.Choices[0].FinishReason {
			case "stop", "length", "content_filter":
				finishReason = envelope.Choices[0].FinishReason
			}
		}
		_, completionTokens, _, _ := extractModelUsageFromResponse(body)
		document = map[string]any{"invalidJSON": true, "contentBytes": len(content), "completionTokens": completionTokens, "finishReason": finishReason}
		kind += "-invalid"
	}
	if strings.HasPrefix(kind, "source-review") && reviewedSource != nil {
		document = map[string]any{
			"sourceHash": fmt.Sprintf("%x", sha256.Sum256([]byte(reviewedSource.AudioScript))),
			"source":     reviewedSource,
			"review":     document,
		}
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return response, nil
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	identifier := key + "-" + kind
	capture.attempts[identifier]++
	name := fmt.Sprintf("private-target-%.0f-%s-%s-%02d.json", capture.band, key, kind, capture.attempts[identifier])
	if os.WriteFile(filepath.Join(capture.directory, name), data, 0600) != nil {
		capture.failed = true
	}
	return response, nil
}

func TestMockExamSourceOutputBudgetCanBeConfigured(t *testing.T) {
	t.Setenv("MOCK_EXAM_SOURCE_MAX_OUTPUT_TOKENS", "120000")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if got := payload["max_output_tokens"]; got != float64(120000) {
			t.Errorf("max_output_tokens = %v, want 120000", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"{\"ok\":true}"}`))
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL+"/v1/responses")
	g.Client = server.Client()
	if _, err := g.callMockExamJSONCompletion(context.Background(), "system", "prompt", "MOCK_EXAM_SOURCE"); err != nil {
		t.Fatal(err)
	}
}

func TestMockExamPrivateCaptureContainsContentOnly(t *testing.T) {
	provider := &mockExamProvider{}
	g := mockExamTestGenerator(t, provider)
	directory := t.TempDir()
	capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: directory, band: 7, attempts: map[string]int{}}
	g.Client.Transport = capture
	if _, err := g.GenerateMockExam(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(directory)
	if err != nil || capture.failed || len(files) != 23 {
		t.Fatal("expected seven source, eight question/task generation and eight review content files")
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"test-key", `"Authorization"`, `"messages"`, `"apiKey"`, `"choices"`} {
			if strings.Contains(string(data), forbidden) {
				t.Fatal("private capture contains request credentials or provider envelope")
			}
		}
		if strings.Contains(file.Name(), "-review-") {
			var review mockExamReview
			if json.Unmarshal(data, &review) != nil || !review.Approved {
				t.Fatal("private capture lost review content")
			}
		} else {
			var draft mockExamDraft
			if err := json.Unmarshal(data, &draft); err != nil || draft.Title == "" {
				t.Fatal("private capture lost generated content")
			}
		}
	}
}

func TestPrivateCaptureDistinguishesGuidanceFromSourceStage(t *testing.T) {
	g := mockExamTestGenerator(t, &mockExamProvider{})
	directory := t.TempDir()
	capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: directory, band: 7, attempts: map[string]int{}}
	g.Client.Transport = capture
	fixture := mockExamFixtureSection(2, 7)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	if _, err := g.generateMockExamQuestions(context.Background(), mockExamSpecs[2], 7, source, mockExamGenerationPrompt(mockExamSpecs[2], 7), "", "", listeningDifficultyGuidance(7)); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(directory)
	if err != nil || capture.failed || len(files) != 2 {
		t.Fatal("expected question generation and blind review only")
	}
	for _, file := range files {
		if strings.Contains(file.Name(), "-source-") {
			t.Fatal("guidance incorrectly classified the question request as source generation")
		}
	}
}

func TestPrivateSpeakingCaptureContainsContentOnly(t *testing.T) {
	g, _ := speakingQualityGenerator(t, speakingQualityEvaluation())
	directory := t.TempDir()
	capture := &mockExamPrivateCapture{base: g.Client.Transport, directory: directory, attempts: map[string]int{}}
	g.Client.Transport = capture
	if _, err := g.EvaluateSpeaking(context.Background(), nil, speakingQualityTurns()); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(directory)
	if err != nil || capture.failed || len(files) != 2 {
		t.Fatal("expected evaluation and factual review captures")
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"offline-fixture", `"Authorization"`, `"messages"`, `"choices"`, `"apiKey"`} {
			if strings.Contains(string(data), forbidden) {
				t.Fatal("speaking diagnostics contain credentials or request/response envelope")
			}
		}
	}
}

func TestPrivateListeningCaptureIncludesIndependentAudits(t *testing.T) {
	for _, tc := range []struct{ marker, kind, response string }{
		{"LISTENING_SOURCE_REVIEW:", "source-review", `{"approved":false,"feedback":"The collection and delivery locations contradict each other."}`},
		{"LISTENING_SOURCE_REVIEW:", "source-review", `{"approved":false,"feedback":"There are conflicting counts in the source.","checks":{"quantities":{"status":"FAIL","reason":"One lower instrument became two lower instruments."}}}`},
		{"LISTENING_SOURCE_REVIEW:", "source-review-invalid", `{"approved":false,"feedback":"unterminated`},
		{"LISTENING_DIFFICULTY_SOURCE:", "difficulty-review", `{"approved":false,"confidence":"HIGH","feedback":"The questions only require direct word retrieval.","items":[]}`},
		{"DIFFICULTY_REPAIR_DATA:", "question-repair", `{"items":[]}`},
		{"LISTENING_FAILED_ITEM_REPAIR:", "question-repair", `{"items":[]}`},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			directory := t.TempDir()
			capture := &mockExamPrivateCapture{directory: directory, band: 6.5, sectionKey: "LISTENING_2", attempts: map[string]int{}}
			capture.base = speakingQualityTransport(func(r *http.Request) (*http.Response, error) {
				body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": tc.response}}}, "privateMetadata": "must-not-persist"})
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
			})
			g := NewOpenAIGenerator("private-secret", "test", "https://test.invalid")
			g.Client.Transport = capture
			source := mockExamSource{Title: "Fixed source", AudioScript: "Maya: The original count is twelve.\nLeo: The final count remains twelve."}
			requestData := "{}"
			if tc.marker == "LISTENING_SOURCE_REVIEW:" {
				encoded, _ := json.Marshal(source)
				requestData = "\n" + string(encoded)
			}
			if _, err := g.mockExamCompletion(context.Background(), "JSON only", tc.marker+requestData, "TEST"); err != nil {
				t.Fatal(err)
			}
			files, err := os.ReadDir(directory)
			if err != nil || capture.failed || len(files) != 1 || !strings.Contains(files[0].Name(), tc.kind) {
				t.Fatal("independent audit was not captured")
			}
			raw, err := os.ReadFile(filepath.Join(directory, files[0].Name()))
			if err != nil || strings.Contains(string(raw), "private-secret") || strings.Contains(string(raw), "must-not-persist") {
				t.Fatal("diagnostic leaked credentials or response metadata")
			}
			if strings.Contains(tc.response, "quantities") && !strings.Contains(string(raw), "One lower instrument became two") {
				t.Fatal("structured source checks were lost in private diagnostics")
			}
			if tc.kind == "source-review-invalid" && (!strings.Contains(string(raw), `"invalidJSON": true`) || strings.Contains(string(raw), "unterminated")) {
				t.Fatal("invalid-response diagnostics must preserve metadata only")
			}
			if tc.marker == "LISTENING_SOURCE_REVIEW:" {
				var evidence struct {
					SourceHash string          `json:"sourceHash"`
					Source     mockExamSource  `json:"source"`
					Review     json.RawMessage `json:"review"`
				}
				wantHash := fmt.Sprintf("%x", sha256.Sum256([]byte(source.AudioScript)))
				if json.Unmarshal(raw, &evidence) != nil || evidence.Source != source || evidence.SourceHash != wantHash || len(evidence.Review) == 0 {
					t.Fatal("source-review capture lost whole-source lineage")
				}
			}
		})
	}
}

func TestMockExamFixtureFile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "mock_exam_valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	var paper domain.MockExam
	if err := json.Unmarshal(data, &paper); err != nil {
		t.Fatal("cannot parse test fixture")
	}
	if err := ValidateMockExamPaper(paper); err != nil {
		t.Fatal(err)
	}
}

// Test-only fixture export for service lifecycle tests; never called in production.
func TestMockExamExportFixture(t *testing.T) {
	if os.Getenv("IELTS_MOCK_EXPORT_FIXTURE") != "1" {
		t.Skip("explicit fixture export only")
	}
	paper := mockExamFixturePaper(7)
	if err := ValidateMockExamPaper(paper); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(paper, "", "  ")
	if err != nil {
		t.Fatal("cannot encode test fixture")
	}
	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("testdata", "mock_exam_valid.json"), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func mockExamSafeReport(paper domain.MockExam, elapsed time.Duration) any {
	type safeQuestion struct {
		Type     string   `json:"type"`
		Question string   `json:"question"`
		Options  []string `json:"options,omitempty"`
	}
	type safeSection struct {
		Key            string                 `json:"key"`
		Title          string                 `json:"title"`
		TargetBand     float64                `json:"targetBand"`
		Approved       bool                   `json:"approved"`
		Instructions   string                 `json:"instructions"`
		Passage        string                 `json:"passage,omitempty"`
		SourceWords    int                    `json:"sourceWords"`
		Questions      []safeQuestion         `json:"questions,omitempty"`
		WritingPrompts []domain.WritingPrompt `json:"writingPrompts,omitempty"`
	}
	sections := make([]safeSection, 0, len(paper.Sections))
	for _, s := range paper.Sections {
		section := safeSection{Key: s.Key, Title: s.Title, TargetBand: s.TargetBand, Approved: s.QualityApproved, Instructions: s.Instructions, Passage: s.Passage, SourceWords: mockExamWordCount(s.Passage + s.AudioScript), WritingPrompts: s.WritingPrompts}
		for _, q := range s.Questions {
			section.Questions = append(section.Questions, safeQuestion{q.Type, q.Question, q.Options})
		}
		sections = append(sections, section)
	}
	return struct {
		Version        string        `json:"version"`
		Notice         string        `json:"notice"`
		ElapsedSeconds float64       `json:"elapsedSeconds"`
		Sections       []safeSection `json:"sections"`
	}{mockExamPaperVersion, "Original AI-generated academic practice; not official IELTS material or calibrated scores. Answer keys, author evidence, reviewer answers and listening scripts are omitted.", elapsed.Seconds(), sections}
}

func TestMockExamSafeReportOmitsSolutions(t *testing.T) {
	p := mockExamFixturePaper(7)
	p.Sections[0].Questions[0].AnswerKey = "SECRET_KEY"
	p.Sections[0].Questions[0].Evidence = "SECRET_EVIDENCE"
	p.Sections[0].AudioScript = "SECRET_SCRIPT"
	data, err := json.Marshal(mockExamSafeReport(p, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"SECRET_", `"answerKey":`, `"audioScript":`, `"evidence":`, `"APIKey":`} {
		if strings.Contains(string(data), field) {
			t.Fatalf("safe report leaked %s", field)
		}
	}
}
