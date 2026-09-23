package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/config"
	"github.com/linguaquest/server/internal/domain"
)

// 只使用自编对照文本；不是官方定分样本，不涉及用户录音，也不调用 TTS。
func TestLiveSpeakingQuality(t *testing.T) {
	if os.Getenv("SPEAKING_QUALITY_LIVE") != "1" {
		t.Skip("explicit opt-in required")
	}
	output := os.Getenv("SPEAKING_QUALITY_OUTPUT_DIR")
	if !filepath.IsAbs(output) {
		t.Fatal("absolute private output directory required")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("cannot resolve server root")
	}
	t.Chdir(root)
	model, _, err := mockExamLiveModelConfig(root, config.Load())
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if os.MkdirAll(output, 0700) != nil {
		t.Fatal("cannot create output")
	}
	output, err = os.MkdirTemp(output, "speaking-quality-")
	if err != nil {
		t.Fatal("cannot create run directory")
	}
	t.Logf("private reports: %s", output)
	g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
	g.UpdateModelConfig(model)
	capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: output, attempts: map[string]int{}}
	g.Client.Transport = capture
	prompts := []domain.SpeakingPrompt{{Part: 1, Question: "What do you enjoy learning and why?"}, {Part: 1, Question: "What difficulties have you encountered?"}}
	samples := []struct{ name, text string }{
		{"limited", "I like learn English because English very useful. Last year I go a club with my friend and we talk English there. I am difficult to understand other people when they speak fast. I not know many word so I say same word many time. Teacher help me and now I can talk little better. I want learning more because my job need English and I hope can speak good with customer."},
		{"developing", "I enjoy learning to cook because it helps me to be more independent. Before I moved away from home, my parents usually prepared the meals, so I didn't have many experience. At first I followed simple recipes on my phone. Sometimes the instructions was confusing and I put too much salt in the food. Now I am more confident with the dishes that I know, although I still find it difficult when a recipe has several things happening at the same time. Last weekend I made a meal for two friends. It wasn't perfect, but they enjoyed it. I want to learn more recipes because eating the same food every week can get boring."},
		{"flexible", "I've recently taken up photography, mainly because it encourages me to notice things I would otherwise walk straight past. I used to assume that a better camera would automatically give me better pictures. However, after attending a weekend workshop, I realised that paying attention to light matters much more. The tutor asked us to photograph the same street at different times of day, which made the difference particularly clear. I still struggle to get a sharp image when people are moving, and some of the technical explanations go over my head. Rather than changing every setting at once, I now experiment with one and compare the results. That makes the process more manageable, and seeing a small improvement gives me a reason to keep practising."},
		{"developed", "What I particularly enjoy is learning to communicate across cultures, rather than simply memorising vocabulary. Last year I joined a conversation group whose members came from several countries, and the experience challenged some assumptions I had taken for granted. Initially, I could follow the main argument but often missed the implication of an indirect remark. Instead of interrupting every time, I started paraphrasing what I thought the speaker meant and asking whether I had understood correctly. That approach made misunderstandings easier to resolve without disrupting the discussion. I still occasionally struggle to choose the most natural collocation, especially under pressure, so I keep a record of expressions that arise in genuine conversations and practise using them in different contexts. Although progress has been gradual, I can now express disagreement more tactfully and explain a complicated point without relying on the same few phrases."},
	}
	results := map[string]domain.SpeakingEvaluation{}
	for _, sample := range samples {
		turns := []domain.SpeakingTurn{{Part: 1, PromptIndex: 0, Prompt: prompts[0].Question, Transcript: sample.text}}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		started := time.Now()
		result, evalErr := g.EvaluateSpeaking(ctx, prompts, turns)
		cancel()
		if capture.failed {
			t.Fatal("cannot persist private stage diagnostics")
		}
		code := ""
		if evalErr != nil {
			// 评估器仅返回固定校验说明及已脱敏的请求错误。
			code = evalErr.Error()
		}
		report := struct {
			Name, Transcript, Failure string
			Seconds                   float64
			Evaluation                domain.SpeakingEvaluation
		}{sample.name, sample.text, code, time.Since(started).Seconds(), result}
		data, _ := json.MarshalIndent(report, "", "  ")
		if os.WriteFile(filepath.Join(output, sample.name+".json"), data, 0600) != nil {
			t.Fatal("cannot save report")
		}
		if evalErr != nil {
			t.Errorf("%s evaluation did not pass validation", sample.name)
			if errors.Is(evalErr, context.DeadlineExceeded) || errors.Is(evalErr, context.Canceled) {
				t.Fatal("model request timed out or was cancelled; report saved, remaining paid samples not attempted")
			}
			continue
		}
		results[sample.name] = result
		t.Logf("%s: text=%.1f lexical=%.1f grammar=%.1f; partial=%t", sample.name, *result.TextCoherence, *result.LexicalResource, *result.GrammarAccuracy, result.IsPartial)
	}
	if len(results) == len(samples) {
		weak, strong := results["limited"], results["developed"]
		if *strong.LexicalResource <= *weak.LexicalResource || *strong.GrammarAccuracy <= *weak.GrammarAccuracy {
			t.Error("clear lexical/grammatical contrast was not distinguished; not a calibration claim")
		}
		// 自编样本没有考官定分标签，只检查明显错误文本与准确灵活文本的相对区分。
		developing, flexible := results["developing"], results["flexible"]
		if *flexible.GrammarAccuracy <= *developing.GrammarAccuracy {
			t.Error("recurring agreement/countability errors were not distinguished from accurate varied grammar")
		}
	}
	turn := domain.SpeakingTurn{Part: 1, PromptIndex: 0, Prompt: prompts[0].Question, Transcript: samples[len(samples)-1].text}
	session := domain.SpeakingSession{PromptIndex: 1, Prompts: prompts, Turns: []domain.SpeakingTurn{turn}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	reply, err := g.NextSpeakingPrompt(ctx, session, turn)
	if err != nil {
		t.Fatal("live examiner reply failed validation")
	}
	data, _ := json.MarshalIndent(reply, "", "  ")
	if os.WriteFile(filepath.Join(output, "examiner.json"), data, 0600) != nil {
		t.Fatal("cannot save examiner result")
	}
	t.Log("live examiner prompt passed text/part/TTS alignment checks")
}
