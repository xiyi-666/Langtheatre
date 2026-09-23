package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/config"
	"github.com/linguaquest/server/internal/domain"
)

// 显式启用才调用已配置的真实模型。样本仅存私有目录，不入库、不发布、不调用 TTS。
func TestLiveListeningDifficulty(t *testing.T) {
	if os.Getenv("LISTENING_DIFFICULTY_LIVE") != "1" {
		t.Skip("explicit opt-in required for paid content validation")
	}
	outputRoot := os.Getenv("LISTENING_DIFFICULTY_OUTPUT_DIR")
	if !filepath.IsAbs(outputRoot) {
		t.Fatal("absolute private output directory required")
	}
	// 与后台默认 40 分钟的任务预算一致，不能用旧的 20 分钟期限误判难度。
	minutes := 40
	if raw := os.Getenv("LISTENING_DIFFICULTY_TIMEOUT_MINUTES"); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil || value < 1 || value > 40 {
			t.Fatal("LISTENING_DIFFICULTY_TIMEOUT_MINUTES must be 1..40")
		}
		minutes = value
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("cannot resolve local server directory")
	}
	t.Chdir(root)
	cfg := config.Load()
	model, _, err := mockExamLiveModelConfig(root, cfg)
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if err = os.MkdirAll(outputRoot, 0700); err != nil {
		t.Fatal("cannot create private directory")
	}
	output, err := os.MkdirTemp(outputRoot, "listening-difficulty-")
	if err != nil {
		t.Fatal("cannot create unique run directory")
	}
	t.Logf("private reports: %s", output)
	caseFilter := strings.TrimSpace(os.Getenv("LISTENING_DIFFICULTY_CASE"))
	matched := false
	for _, sample := range []struct {
		part int
		band float64
	}{{1, 6}, {1, 6.5}, {2, 6.5}, {3, 6.5}, {4, 6.5}, {3, 7}, {3, 7.5}, {4, 8}} {
		caseID := fmt.Sprintf("%d:%.1f", sample.part, sample.band)
		if caseFilter != "" && caseFilter != caseID {
			continue
		}
		matched = true
		stopForServiceFailure := false
		t.Run(fmt.Sprintf("Part%d-target%.1f", sample.part, sample.band), func(t *testing.T) {
			g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
			g.UpdateModelConfig(model)
			capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: output, band: sample.band, sectionKey: mockExamSpecs[sample.part-1].key, attempts: map[string]int{}}
			g.Client.Transport = capture
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(minutes)*time.Minute)
			defer cancel()
			started := time.Now()
			section, generationErr := g.GenerateListeningTraining(ctx, sample.part, sample.band)
			if capture.failed {
				t.Fatal("cannot persist private stage diagnostics")
			}
			validationErr := ValidateListeningTraining(section, sample.part)
			stopForServiceFailure = errors.Is(generationErr, context.DeadlineExceeded) || errors.Is(generationErr, context.Canceled) || errors.Is(generationErr, ErrListeningReviewUnavailable)
			approved := generationErr == nil && validationErr == nil
			reason := ""
			if generationErr != nil {
				reason = "generation_or_review_failed"
				switch {
				case errors.Is(generationErr, context.DeadlineExceeded), errors.Is(generationErr, context.Canceled):
					reason = "generation_or_review_deadline"
				case errors.Is(generationErr, ErrListeningReviewUnavailable):
					reason = "difficulty_review_unavailable"
				case errors.Is(generationErr, ErrListeningReviewInvalid):
					reason = "difficulty_review_invalid_response"
				case section.ListeningDifficulty != nil:
					// 此处校验只返回固定规则诊断，不包含原始供应商响应。
					if gateErr := ValidateListeningDifficulty(section); gateErr != nil {
						reason = gateErr.Error()
					}
				}
			} else if validationErr != nil {
				reason = validationErr.Error()
			}
			report := struct {
				Part           int
				Target         float64
				Approved       bool
				ElapsedSeconds float64
				Reason         string
				Section        domain.MockExamSection
			}{sample.part, sample.band, approved, time.Since(started).Seconds(), reason, section}
			data, encodeErr := json.MarshalIndent(report, "", "  ")
			if encodeErr != nil || os.WriteFile(filepath.Join(output, fmt.Sprintf("part%d-target%.1f.json", sample.part, sample.band)), data, 0600) != nil {
				t.Fatal("cannot persist private validation report")
			}
			t.Logf("part=%d target=%.1f approved=%v elapsed=%.0fs reason=%s", sample.part, sample.band, approved, report.ElapsedSeconds, reason)
			if !approved {
				t.Error("sample is NOT cleared for release")
			}
		})
		if stopForServiceFailure {
			t.Fatal("service failure saved in private report; remaining paid samples not attempted")
		}
	}
	if !matched {
		t.Fatal("LISTENING_DIFFICULTY_CASE must match a configured part:band sample")
	}
}

// 门禁升级后复核私有历史样本，不发起模型请求、不更改原报告的审批结果。
func TestRevalidateListeningReports(t *testing.T) {
	directory := os.Getenv("LISTENING_REVALIDATE_DIR")
	if directory == "" {
		t.Skip("explicit private report directory required")
	}
	if !filepath.IsAbs(directory) {
		t.Fatal("absolute private directory required")
	}
	paths, err := filepath.Glob(filepath.Join(directory, "part*-target*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatal("no validation reports found")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Part     int
				Approved bool
				Section  domain.MockExamSection
			}
			if json.Unmarshal(data, &report) != nil {
				t.Fatal("invalid report")
			}
			if !report.Approved {
				t.Skip("original generation already rejected; no release claim")
			}
			if err := ValidateListeningTraining(report.Section, report.Part); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 对审核响应不一致的私有失败报告重新盲审；不修改原稿、题目或答案。
func TestLiveListeningRereview(t *testing.T) {
	if os.Getenv("LISTENING_DIFFICULTY_LIVE") != "1" {
		t.Skip("explicit paid validation opt-in required")
	}
	path, outputRoot := os.Getenv("LISTENING_REREVIEW_REPORT"), os.Getenv("LISTENING_DIFFICULTY_OUTPUT_DIR")
	if !filepath.IsAbs(path) || !filepath.IsAbs(outputRoot) {
		t.Fatal("absolute failed report and private output directory required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Part     int
		Target   float64
		Approved bool
		Section  domain.MockExamSection
	}
	if json.Unmarshal(data, &report) != nil || report.Approved || report.Part < 1 || report.Part > 4 || report.Target != report.Section.TargetBand {
		t.Fatal("expected an unpublished failed listening report")
	}
	if err = validateMockExamSection(report.Section, mockExamSpecs[report.Part-1]); err != nil {
		t.Fatal("report did not pass deterministic and answer-quality gates")
	}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	model, _, err := mockExamLiveModelConfig(root, config.Load())
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if os.MkdirAll(outputRoot, 0700) != nil {
		t.Fatal("cannot create private directory")
	}
	output, err := os.MkdirTemp(outputRoot, "listening-rereview-")
	if err != nil {
		t.Fatal(err)
	}
	g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
	g.UpdateModelConfig(model)
	capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: output, band: report.Target, sectionKey: report.Section.Key, attempts: map[string]int{}}
	g.Client.Transport = capture
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	err = g.reviewListeningDifficulty(ctx, &report.Section)
	if err == nil {
		err = ValidateListeningTraining(report.Section, report.Part)
	}
	result := struct {
		Part     int
		Target   float64
		Approved bool
		Reason   string
		Section  domain.MockExamSection
	}{report.Part, report.Target, err == nil, "", report.Section}
	if err != nil {
		result.Reason = err.Error()
	}
	encoded, encodeErr := json.MarshalIndent(result, "", "  ")
	if encodeErr != nil || os.WriteFile(filepath.Join(output, fmt.Sprintf("part%d-target%.1f.json", report.Part, report.Target)), encoded, 0600) != nil || capture.failed {
		t.Fatal("cannot persist private rereview report")
	}
	t.Logf("private reports: %s", output)
	if err != nil {
		t.Fatal(err)
	}
}

// 针对失败的真实样本做有界修题，保留原文；不是重新抽样直到碰到通过结果。
func TestLiveListeningRepair(t *testing.T) {
	if os.Getenv("LISTENING_DIFFICULTY_LIVE") != "1" {
		t.Skip("explicit paid validation opt-in required")
	}
	path, outputRoot := os.Getenv("LISTENING_REPAIR_REPORT"), os.Getenv("LISTENING_DIFFICULTY_OUTPUT_DIR")
	if !filepath.IsAbs(path) || !filepath.IsAbs(outputRoot) {
		t.Fatal("absolute failed report and private output directory required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var original struct {
		Part     int
		Target   float64
		Approved bool
		Section  domain.MockExamSection
	}
	if json.Unmarshal(data, &original) != nil || original.Part < 1 || original.Part > 4 || !validMockExamBand(original.Target) || original.Target != original.Section.TargetBand || original.Approved {
		t.Fatal("expected a real unpublished listening report with its frozen source")
	}
	spec := mockExamSpecs[original.Part-1]
	if err := validateMockExamSection(original.Section, spec); err != nil {
		t.Fatal("original sample must have passed structural/answer review before difficulty rejection")
	}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	model, _, err := mockExamLiveModelConfig(root, config.Load())
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if os.MkdirAll(outputRoot, 0700) != nil {
		t.Fatal("cannot create private directory")
	}
	output, err := os.MkdirTemp(outputRoot, "listening-repair-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("private reports: %s", output)
	g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
	g.UpdateModelConfig(model)
	capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: output, band: original.Target, sectionKey: spec.key, attempts: map[string]int{}}
	g.Client.Transport = capture
	source := mockExamSource{Title: original.Section.Title, AudioScript: original.Section.AudioScript}
	previous, _ := json.Marshal(mockExamDraft{Instructions: original.Section.Instructions, Questions: original.Section.Questions})
	feedback := "Previous authored questions failed a production quality gate. Rebuild the set from its frozen source and binding high-band plan."
	fixed := map[int]domain.QuizQuestion(nil)
	if original.Section.ListeningDifficulty != nil {
		feedback = listeningRevisionFeedback(original.Section, ErrListeningDifficulty)
		fixed = listeningRevisionFixedItems(original.Section)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	started := time.Now()
	var plan *listeningQuestionPlan
	activeGuidance := listeningDifficultyGuidance(original.Target)
	if original.Target >= 7.5 {
		planCtx, planCancel := context.WithTimeout(ctx, 10*time.Minute)
		planned, planErr := g.planHighBandListeningQuestions(planCtx, source, original.Part, original.Target, feedback)
		planCancel()
		if planErr != nil {
			t.Fatal(planErr)
		}
		plan = &planned
		activeGuidance += "\n" + listeningQuestionPlanGuidance(planned)
		fixed = nil
	}
	questionsCtx, questionsCancel := context.WithTimeout(ctx, 10*time.Minute)
	section, generationErr := g.generateMockExamQuestionsWithFixedItemsAndPlan(questionsCtx, spec, original.Target, source, listeningTrainingQuestionPrompt(original.Part, original.Target), feedback, string(previous), fixed, plan, activeGuidance)
	questionsCancel()
	if generationErr == nil {
		reviewCtx, reviewCancel := context.WithTimeout(ctx, 4*time.Minute)
		generationErr = g.reviewListeningDifficulty(reviewCtx, &section)
		reviewCancel()
	}
	if generationErr == nil {
		generationErr = ValidateListeningTraining(section, original.Part)
	}
	reason := ""
	if generationErr != nil {
		reason = generationErr.Error()
	}
	report := struct {
		Part           int
		Target         float64
		Approved       bool
		ElapsedSeconds float64
		Reason         string
		Section        domain.MockExamSection
	}{original.Part, original.Target, generationErr == nil, time.Since(started).Seconds(), reason, section}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(output, fmt.Sprintf("part%d-target%.1f.json", original.Part, original.Target)), encoded, 0600) != nil || capture.failed {
		t.Fatal("cannot persist private report")
	}
	if generationErr != nil {
		t.Fatal(generationErr)
	}
	if section.AudioScript != original.Section.AudioScript {
		t.Fatal("repair changed frozen source")
	}
	t.Logf("same-source repair cleared internal gates in %.0fs; not an official calibration", report.ElapsedSeconds)
}

// 从已保存的私有原稿继续完整验收，避免因后续题目门禁变化反复付费生成原稿。
func TestLiveListeningFromSource(t *testing.T) {
	if os.Getenv("LISTENING_DIFFICULTY_LIVE") != "1" {
		t.Skip("explicit paid validation opt-in required")
	}
	path, outputRoot := os.Getenv("LISTENING_SOURCE_FILE"), os.Getenv("LISTENING_DIFFICULTY_OUTPUT_DIR")
	if !filepath.IsAbs(path) || !filepath.IsAbs(outputRoot) {
		t.Fatal("absolute source file and private output directory required")
	}
	part, partErr := strconv.Atoi(os.Getenv("LISTENING_SOURCE_PART"))
	band, bandErr := strconv.ParseFloat(os.Getenv("LISTENING_SOURCE_BAND"), 64)
	if partErr != nil || bandErr != nil || part < 1 || part > 4 || !validMockExamBand(band) {
		t.Fatal("valid LISTENING_SOURCE_PART and LISTENING_SOURCE_BAND required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var source mockExamSource
	if json.Unmarshal(data, &source) != nil {
		t.Fatal("invalid private source JSON")
	}
	spec := mockExamSpecs[part-1]
	if err = validateMockExamSource(source, spec); err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	model, _, err := mockExamLiveModelConfig(root, config.Load())
	if err != nil || model.APIKey == "" {
		t.Fatal("live model configuration unavailable")
	}
	if os.MkdirAll(outputRoot, 0700) != nil {
		t.Fatal("cannot create private directory")
	}
	output, err := os.MkdirTemp(outputRoot, "listening-from-source-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("private reports: %s", output)
	g := NewOpenAIGenerator(model.APIKey, model.Model, model.BaseURL)
	g.UpdateModelConfig(model)
	capture := &mockExamPrivateCapture{base: http.DefaultTransport, directory: output, band: band, sectionKey: spec.key, attempts: map[string]int{}}
	g.Client.Transport = capture
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	started := time.Now()
	guidance := listeningDifficultyGuidance(band)
	source, err = g.reviewAndRepairListeningSource(ctx, spec, band, mockExamSourcePrompt(spec, band), guidance, source)
	var section domain.MockExamSection
	if err == nil {
		var plan listeningQuestionPlan
		plan, err = g.planHighBandListeningQuestions(ctx, source, part, band, "")
		if err == nil {
			activeGuidance := guidance + "\n" + listeningQuestionPlanGuidance(plan)
			section, err = g.generateMockExamQuestionsWithFixedItemsAndPlan(ctx, spec, band, source, listeningTrainingQuestionPrompt(part, band), "", "", nil, &plan, activeGuidance)
		}
	}
	if err == nil {
		err = g.reviewListeningDifficulty(ctx, &section)
	}
	if err == nil {
		err = ValidateListeningTraining(section, part)
	}
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	report := struct {
		Part           int
		Target         float64
		Approved       bool
		ElapsedSeconds float64
		Reason         string
		Section        domain.MockExamSection
	}{part, band, err == nil, time.Since(started).Seconds(), reason, section}
	encoded, encodeErr := json.MarshalIndent(report, "", "  ")
	if encodeErr != nil || os.WriteFile(filepath.Join(output, fmt.Sprintf("part%d-target%.1f.json", part, band)), encoded, 0600) != nil || capture.failed {
		t.Fatal("cannot persist private report")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("source-backed sample cleared all internal gates in %.0fs; not an official calibration", report.ElapsedSeconds)
}
