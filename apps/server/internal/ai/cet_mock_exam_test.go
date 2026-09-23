package ai

import (
	"fmt"
	"github.com/linguaquest/server/internal/domain"
	"strings"
	"testing"
)

// 仅验证结构边界的合成夹具，不是可发布的试题；语义质量仍须独立模型审核。
func cetStructuralFixture(exam string) domain.MockExam {
	specs := cetMockExamSpecs[exam]
	paper := domain.MockExam{Exam: exam, TotalDurationSeconds: cetMockExamTotalSeconds(specs)}
	evidence := "The visitors discussed the local development plans carefully"
	bank := strings.Fields("adapt balance create decline expand focus generate handle improve justify keep learn manage notice observe")
	for _, spec := range specs {
		s := domain.MockExamSection{Key: spec.key, Skill: spec.skill, Title: "Practice material", Instructions: "Read the instructions and answer each question carefully.", DurationSeconds: spec.seconds, PaperVersion: CETMockExamPaperVersion(exam), QualityApproved: true}
		switch spec.skill {
		case "WRITING":
			s.WritingPrompts = []domain.WritingPrompt{{Title: "Practice essay", Instructions: "Write an essay discussing the importance of education in society. Explain your viewpoint and provide supporting reasons in your answer.", SuggestedWordCount: 150}}
		case "TRANSLATION":
			n := 140
			if exam == "CET6" {
				n = 180
			}
			s.WritingPrompts = []domain.WritingPrompt{{Title: "Translate paragraph", Instructions: "Translate the following Chinese paragraph into clear English, preserving its meaning and logical relationships. " + strings.Repeat("中", n)}}
		case "LISTENING":
			groups := []string{"News 1", "News 2", "News 3", "Conversation 1", "Conversation 2", "Passage 1", "Passage 2", "Passage 3"}
			counts := []int{2, 2, 3, 4, 4, 3, 3, 4}
			if exam == "CET6" {
				groups = []string{"Conversation 1", "Conversation 2", "Passage 1", "Passage 2", "Lecture 1", "Lecture 2", "Lecture 3"}
				counts = []int{4, 4, 3, 4, 3, 3, 4}
			}
			n := 0
			repetitions := 25
			if exam == "CET6" {
				repetitions = 32
			}
			for g, group := range groups {
				s.AudioScript += "Narrator: " + group + "\nSpeaker: " + strings.Repeat(evidence+". ", repetitions) + "\n"
				for j := 0; j < counts[g]; j++ {
					n++
					question := fmt.Sprintf("Why did visitor number %d discuss the local plans?", n)
					s.AudioScript += fmt.Sprintf("Narrator: Question %d. %s\n", n, question)
					s.Questions = append(s.Questions, domain.QuizQuestion{Type: "multiple_choice", Question: question, Options: []string{"A. To review proposals", "B. To cancel appointments", "C. To deliver equipment", "D. To hire workers"}, AnswerKey: "A", Evidence: evidence})
				}
			}
		case "READING":
			s.Passage = "Cloze\n" + strings.Repeat(evidence+". ", 25)
			for i := 1; i <= 10; i++ {
				s.Passage += fmt.Sprintf(" [%d] ", i)
			}
			s.Passage += "\nMatching\n"
			opts := []string{}
			for i := 0; i < 10; i++ {
				opts = append(opts, fmt.Sprintf("%c. Paragraph %c", 'A'+i, 'A'+i))
				s.Passage += fmt.Sprintf("%c. %s\n", 'A'+i, strings.Repeat(evidence+". ", 12))
			}
			s.Passage += "Passage 1\n" + strings.Repeat(evidence+". ", 50) + "\nPassage 2\n" + strings.Repeat(evidence+". ", 50)
			for i := 0; i < 30; i++ {
				q := domain.QuizQuestion{Question: fmt.Sprintf("Select the appropriate answer for item %d in the passage.", i+1), Evidence: evidence}
				if i < 10 {
					q.Type = "word_bank"
					q.Options = bank
					q.AnswerKey = bank[i]
				} else if i < 20 {
					q.Type = "matching"
					q.Options = opts
					q.AnswerKey = "A"
				} else {
					q.Type = "multiple_choice"
					q.Options = []string{"A. Planning carefully", "B. Buying equipment", "C. Ignoring requests", "D. Closing early"}
					q.AnswerKey = "A"
				}
				s.Questions = append(s.Questions, q)
			}
		}
		paper.Sections = append(paper.Sections, s)
	}
	return paper
}

func TestCETPaperStructureAndRejections(t *testing.T) {
	for _, exam := range []string{"CET4", "CET6"} {
		paper := cetStructuralFixture(exam)
		if err := ValidateCETMockExamPaper(paper); err != nil {
			t.Fatalf("%s fixture: %v", exam, err)
		}
		paper.Sections[1].Questions[0].Type = "sentence_completion"
		if ValidateCETMockExamPaper(paper) == nil {
			t.Fatal("IELTS-style listening accepted")
		}
		paper = cetStructuralFixture(exam)
		paper.Sections[2].Questions[0].Type = "multiple_choice"
		if ValidateCETMockExamPaper(paper) == nil {
			t.Fatal("missing word bank accepted")
		}
		paper = cetStructuralFixture(exam)
		paper.Sections[1].AudioScript = strings.Replace(paper.Sections[1].AudioScript, "Question 25.", "Question missing.", 1)
		if ValidateCETMockExamPaper(paper) == nil {
			t.Fatal("missing spoken question accepted")
		}
		paper = cetStructuralFixture(exam)
		paper.Sections[3].WritingPrompts[0].Instructions = "No source paragraph is available here for the candidate to translate into English."
		if ValidateCETMockExamPaper(paper) == nil {
			t.Fatal("translation without Chinese source accepted")
		}
	}
}

func TestCETWordBankCanonicalAnswer(t *testing.T) {
	q := domain.QuizQuestion{Type: "word_bank", Options: []string{"adapt", "balance"}}
	if mockExamCanonicalAnswer(q, "balance") != "balance" {
		t.Fatal("blind reviewer cannot compare word-bank answers")
	}
}
