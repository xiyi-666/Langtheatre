package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/config"
	"github.com/linguaquest/server/internal/domain"
)

type listeningTextFixedInput struct {
	file       string
	part       int
	target     float64
	sectionKey string
	reportHash string
	audioHash  string
}

type listeningTextInputReport struct {
	Part            int
	Target          float64
	Subject         string
	StructurePassed bool
	Section         domain.MockExamSection
}

var listeningTextFixedInputs = []listeningTextFixedInput{
	{"part3-target7.5.json", 3, 7.5, "LISTENING_3", "eb6b3a89dafe6eff880908f5a20199b44e323fd590583262a285e618079ac31e", "85d7c0080f5471332231753be99f6957562f5eaf610406c8c131225e894301ba"},
	{"part4-target8.0.json", 4, 8, "LISTENING_4", "d85b41d70701e07cf507eb0cabdcc96f5fc549841c4932101768839b08e732e9", "49244a55f6068ded7df1374ef3e5727c1999ec3c487e1608d10c0afd4d718060"},
}

func listeningTextSHA256(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func loadListeningTextFixedInput(inputDir string, fixed listeningTextFixedInput) (listeningTextInputReport, error) {
	var report listeningTextInputReport
	data, err := os.ReadFile(filepath.Join(inputDir, fixed.file))
	if err != nil {
		return report, err
	}
	if got := listeningTextSHA256(data); got != fixed.reportHash {
		return report, fmt.Errorf("fixed input report hash mismatch for %s: %s", fixed.file, got)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return report, err
	}
	if report.Part != fixed.part || report.Target != fixed.target || report.Section.Key != fixed.sectionKey || !report.StructurePassed || !strings.HasPrefix(report.Subject, "Assigned subject:") {
		return report, fmt.Errorf("fixed input metadata mismatch for %s", fixed.file)
	}
	if got := listeningTextSHA256([]byte(report.Section.AudioScript)); got != fixed.audioHash {
		return report, fmt.Errorf("fixed input script hash mismatch for %s: %s", fixed.file, got)
	}
	return report, nil
}

func TestListeningTextFixedInputsUnchanged(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	inputDir := filepath.Join(root, "..", "..", ".workflow", "ielts-runtime", "automatic-text-sampling-20260916", "fresh-text-1687051642")
	for _, fixed := range listeningTextFixedInputs {
		if _, err := loadListeningTextFixedInput(inputDir, fixed); err != nil {
			t.Fatal(err)
		}
	}
}

// 抽检当前生成器的原文阶段，不生成题目、音频或人工修订稿；默认不调用付费模型。
func TestLiveListeningTextSamples(t *testing.T) {
	if os.Getenv("LISTENING_TEXT_LIVE") != "1" {
		t.Skip("explicit opt-in required for paid text sampling")
	}
	outputRoot := os.Getenv("LISTENING_TEXT_OUTPUT_DIR")
	reviewed := os.Getenv("LISTENING_TEXT_REVIEWED") == "1"
	direct := os.Getenv("LISTENING_TEXT_DIRECT") == "1"
	if !filepath.IsAbs(outputRoot) {
		t.Fatal("absolute private output directory required")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("cannot locate server directory")
	}
	t.Chdir(root)
	model, configSource, err := mockExamLiveModelConfig(root, config.Load())
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if err = os.MkdirAll(outputRoot, 0700); err != nil {
		t.Fatal("cannot create private directory")
	}
	output, err := os.MkdirTemp(outputRoot, "fresh-text-")
	if err != nil {
		t.Fatal("cannot create unique run directory")
	}
	t.Logf("private reports: %s", output)
	seenSubjects := map[string]bool{}
	for _, sample := range []struct {
		part int
		band float64
	}{{1, 6}, {2, 6.5}, {3, 7}, {3, 7.5}, {4, 8}} {
		stop := false
		t.Run(fmt.Sprintf("Part%d-target%.1f", sample.part, sample.band), func(t *testing.T) {
			spec := mockExamSpecs[sample.part-1]
			prompt, subject := "", ""
			// 仅在请求前选择不同主题；不按生成质量重新抽样。
			for pick := 0; pick < 100; pick++ {
				prompt = mockExamSourcePrompt(spec, sample.band)
				for _, line := range strings.Split(prompt, "\n") {
					if strings.HasPrefix(line, "Assigned subject:") {
						subject = line
						break
					}
				}
				if subject != "" && !seenSubjects[subject] {
					break
				}
			}
			if subject == "" || seenSubjects[subject] {
				t.Fatal("could not select a distinct topic before generation")
			}
			seenSubjects[subject] = true
			g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
			g.UpdateModelConfig(model)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			if direct {
				transport.Proxy = nil
			}
			defer transport.CloseIdleConnections()
			capture := &mockExamPrivateCapture{base: transport, directory: output, band: sample.band, sectionKey: spec.key, attempts: map[string]int{}}
			g.Client.Transport = capture
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			started := time.Now()
			source, generationErr := g.generateMockExamSourceWithPrompt(ctx, spec, prompt, listeningDifficultyGuidance(sample.band))
			if reviewed && generationErr == nil {
				source, generationErr = g.reviewAndRepairListeningSource(ctx, spec, sample.band, prompt, listeningDifficultyGuidance(sample.band), source)
			}
			cancel()
			failure := ""
			if generationErr != nil {
				failure = "source_generation_or_structure_failed"
				if errors.Is(generationErr, context.DeadlineExceeded) || errors.Is(generationErr, context.Canceled) {
					failure, stop = "source_request_deadline", true
				}
			}
			capture.mu.Lock()
			attempts, captureFailed := capture.attempts[spec.key+"-source"], capture.failed
			capture.mu.Unlock()
			report := struct {
				Part                                                int
				Target                                              float64
				Model, ConfigurationSource, Scope, Subject, Failure string
				StructurePassed                                     bool
				SourceAttempts                                      int
				ElapsedSeconds                                      float64
				Section                                             domain.MockExamSection
			}{sample.part, sample.band, model.Model, configSource, "FRESH_AUTOMATIC_SOURCE_ONLY", subject, failure, generationErr == nil, attempts, time.Since(started).Seconds(), domain.MockExamSection{Key: spec.key, Title: source.Title, AudioScript: source.AudioScript}}
			if reviewed {
				report.Scope = "FRESH_AUTOMATIC_SOURCE_WITH_PRODUCTION_REVIEW"
			}
			data, encodeErr := json.MarshalIndent(report, "", "  ")
			if encodeErr != nil || captureFailed || os.WriteFile(filepath.Join(output, fmt.Sprintf("part%d-target%.1f.json", sample.part, sample.band)), data, 0600) != nil {
				t.Fatal("cannot persist complete private diagnostics")
			}
			t.Logf("structure=%t attempts=%d words=%d seconds=%.0f failure=%s", report.StructurePassed, attempts, mockExamWordCount(source.AudioScript), report.ElapsedSeconds, failure)
			if generationErr != nil {
				t.Error("source not ready for independent text audit")
			}
		})
		if stop {
			t.Fatal("request deadline recorded; remaining paid samples not attempted")
		}
	}
}

// 使用同一份问题原稿运行生产审核和有界修订，不手工改稿、不重新抽题。
func TestLiveListeningTextRepair(t *testing.T) {
	if os.Getenv("LISTENING_TEXT_LIVE") != "1" {
		t.Skip("explicit paid text validation opt-in required")
	}
	inputDir, outputRoot := os.Getenv("LISTENING_TEXT_INPUT_DIR"), os.Getenv("LISTENING_TEXT_OUTPUT_DIR")
	direct := os.Getenv("LISTENING_TEXT_DIRECT") == "1"
	if !filepath.IsAbs(inputDir) || !filepath.IsAbs(outputRoot) {
		t.Fatal("absolute private input and output directories required")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	expectedInputDir := filepath.Join(root, "..", "..", ".workflow", "ielts-runtime", "automatic-text-sampling-20260916", "fresh-text-1687051642")
	if !strings.EqualFold(filepath.Clean(inputDir), filepath.Clean(expectedInputDir)) {
		t.Fatalf("fixed input directory required: %s", expectedInputDir)
	}
	validated := make(map[string]listeningTextInputReport, len(listeningTextFixedInputs))
	for _, fixed := range listeningTextFixedInputs {
		original, loadErr := loadListeningTextFixedInput(inputDir, fixed)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		validated[fixed.file] = original
	}
	t.Chdir(root)
	model, _, err := mockExamLiveModelConfig(root, config.Load())
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if os.MkdirAll(outputRoot, 0700) != nil {
		t.Fatal("cannot create private directory")
	}
	output, err := os.MkdirTemp(outputRoot, "source-repair-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("private reports: %s", output)
	for _, fixed := range listeningTextFixedInputs {
		stop := false
		t.Run(strings.TrimSuffix(fixed.file, ".json"), func(t *testing.T) {
			original := validated[fixed.file]
			spec := mockExamSpecs[original.Part-1]
			prompt := mockExamSourcePrompt(spec, original.Target)
			lines := strings.Split(prompt, "\n")
			for i, line := range lines {
				if strings.HasPrefix(line, "Assigned subject:") {
					lines[i] = original.Subject
				}
			}
			g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
			g.UpdateModelConfig(model)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			if direct {
				transport.Proxy = nil
			}
			defer transport.CloseIdleConnections()
			capture := &mockExamPrivateCapture{base: transport, directory: output, band: original.Target, sectionKey: spec.key, attempts: map[string]int{}}
			g.Client.Transport = capture
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			started := time.Now()
			source, reviewErr := g.reviewAndRepairListeningSource(ctx, spec, original.Target, strings.Join(lines, "\n"), listeningDifficultyGuidance(original.Target), mockExamSource{Title: original.Section.Title, AudioScript: original.Section.AudioScript})
			failure := ""
			if reviewErr != nil {
				failure = "source_review_or_repair_failed"
				if errors.Is(reviewErr, ErrListeningReviewInvalid) {
					failure = "source_review_invalid_response"
				}
				stop = errors.Is(reviewErr, ErrListeningReviewUnavailable) || ctx.Err() != nil
				if stop {
					failure = "source_review_service_unavailable_or_timeout"
				}
			}
			report := struct {
				Part                                                                 int
				Target                                                               float64
				Scope, Input, InputReportSHA256, InputScriptSHA256, Subject, Failure string
				StructurePassed                                                      bool
				ElapsedSeconds                                                       float64
				Section                                                              domain.MockExamSection
			}{original.Part, original.Target, "SAME_SOURCE_PRODUCTION_REVIEW_AND_REPAIR", filepath.Join(inputDir, fixed.file), fixed.reportHash, fixed.audioHash, original.Subject, failure, reviewErr == nil, time.Since(started).Seconds(), domain.MockExamSection{Key: spec.key, Title: source.Title, AudioScript: source.AudioScript}}
			encoded, err := json.MarshalIndent(report, "", "  ")
			capture.mu.Lock()
			captureFailed := capture.failed
			capture.mu.Unlock()
			if err != nil || captureFailed || os.WriteFile(filepath.Join(output, fixed.file), encoded, 0600) != nil {
				t.Fatal("cannot persist complete private report")
			}
			t.Logf("approved=%t words=%d seconds=%.0f failure=%s", report.StructurePassed, mockExamWordCount(source.AudioScript), report.ElapsedSeconds, failure)
			if reviewErr != nil {
				t.Error("same source has not passed production source review")
			}
		})
		if stop {
			t.Fatal("service failure recorded; remaining paid samples not attempted")
		}
	}
}
