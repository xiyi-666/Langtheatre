package ai

import (
	"context"
	"encoding/json"
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

// 默认跳过；仅主代理显式开启后调用付费模型，不写数据库。
// IELTS_MOCK_SECTIONS_LIVE=1
// IELTS_MOCK_SECTIONS=READING_1,READING_2,READING_3,WRITING（也支持 LISTENING_3）
// IELTS_MOCK_SECTIONS_BAND=7（6..8，半分步长）
// IELTS_MOCK_SECTIONS_OUTPUT_DIR 必须是用户指定的绝对私有目录。
// 在 apps/server 执行：
// go test ./internal/ai -run '^TestLiveMockExamSections$' -v -count=1 -timeout=55m
// 每次创建独立子目录，保留完整 section、生成草稿、审核 DTO 和统计。
func TestLiveMockExamSections(t *testing.T) {
	if os.Getenv("IELTS_MOCK_SECTIONS_LIVE") != "1" {
		t.Skip("set IELTS_MOCK_SECTIONS_LIVE=1 to run paid section validation")
	}
	// 在加载应用 .env 之前读取显式测试参数，避免 .env 隐式开启或改变测试范围。
	outputRoot := os.Getenv("IELTS_MOCK_SECTIONS_OUTPUT_DIR")
	if !filepath.IsAbs(outputRoot) {
		t.Fatal("an explicit absolute private IELTS_MOCK_SECTIONS_OUTPUT_DIR is required")
	}
	selection := os.Getenv("IELTS_MOCK_SECTIONS")
	if selection == "" {
		selection = "READING_1,READING_2,READING_3,WRITING"
	}
	bandText := os.Getenv("IELTS_MOCK_SECTIONS_BAND")
	if bandText == "" {
		bandText = "7"
	}
	band, err := strconv.ParseFloat(strings.TrimSpace(bandText), 64)
	if err != nil || !validMockExamBand(band) {
		t.Fatal("IELTS_MOCK_SECTIONS_BAND must be 6..8 in half steps")
	}
	var specs []mockExamSpec
	seen := make(map[string]bool)
	for _, item := range strings.Split(selection, ",") {
		key := strings.ToUpper(strings.TrimSpace(item))
		switch key {
		case "READING_1", "READING_2", "READING_3", "WRITING", "LISTENING_1", "LISTENING_2", "LISTENING_3", "LISTENING_4":
		default:
			t.Fatal("IELTS_MOCK_SECTIONS supports LISTENING_1..4, READING_1..3 and WRITING only")
		}
		if seen[key] {
			continue // 重复选择不产生额外付费调用。
		}
		seen[key] = true
		found := false
		for _, spec := range mockExamSpecs {
			if spec.key == key {
				specs = append(specs, spec)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("selected section has no application specification")
		}
	}
	serverRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("cannot resolve server root")
	}
	// 与应用启动目录和 .env 优先级一致；Chdir 在测试结束时自动恢复。
	t.Chdir(serverRoot)
	cfg := config.Load()
	if cfg.DatabaseURL != "" {
		t.Fatal("section live harness supports local SQLite or environment configuration only")
	}
	modelConfig, configSource, err := mockExamLiveModelConfig(serverRoot, cfg)
	if err != nil {
		t.Fatal("cannot resolve live model configuration")
	}
	if strings.TrimSpace(modelConfig.APIKey) == "" {
		t.Fatal("live section validation requires a configured model API key")
	}
	if err := os.MkdirAll(outputRoot, 0700); err != nil {
		t.Fatal("cannot create private output directory")
	}
	output, err := os.MkdirTemp(outputRoot, "ielts-sections-")
	if err != nil {
		t.Fatal("cannot create independent private artifact directory")
	}
	t.Logf("private artifacts: %s", output)
	// 有意串行执行，不调用 Parallel，也不共享取消上下文；任一失败后继续其他节。
	for _, spec := range specs {
		t.Run(spec.key, func(t *testing.T) {
			g := NewOpenAIGenerator(modelConfig.APIKey, modelConfig.Model, modelConfig.BaseURL)
			g.UpdateModelConfig(modelConfig)
			capture := &mockExamPrivateCapture{
				base: http.DefaultTransport, directory: output, band: band, attempts: map[string]int{},
			}
			g.Client.Transport = capture
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			started := time.Now()
			section, generationErr := g.generateMockExamSection(ctx, spec, band)
			elapsed := time.Since(started).Seconds()
			report := mockExamSectionLiveReport{
				Model: g.modelName(), ConfigurationSource: configSource,
				SectionKey: spec.key, TargetBand: band, ElapsedSeconds: elapsed,
				Section: section, CountsSource: "returned_section",
				Approved: generationErr == nil && section.QualityApproved,
			}
			content := mockExamDraft{Passage: section.Passage, AudioScript: section.AudioScript,
				Questions: section.Questions, WritingPrompts: section.WritingPrompts}
			capture.mu.Lock()
			captureFailed := capture.failed
			lastAttempt := capture.attempts[spec.key+"-generation"]
			lastSourceAttempt := capture.attempts[spec.key+"-source"]
			capture.mu.Unlock()
			if generationErr != nil {
				// 仅固定错误文本进入报告，禁止记录供应商错误、配置或完整请求。
				report.Error = "section generation or review failed"
				report.CountsSource = "unavailable"
				if lastAttempt > 0 {
					name := fmt.Sprintf("private-target-%.0f-%s-generation-%02d.json", band, spec.key, lastAttempt)
					data, readErr := os.ReadFile(filepath.Join(output, name))
					if readErr == nil && json.Unmarshal(data, &content) == nil {
						report.CountsSource = "last_captured_draft"
					} else {
						t.Error("cannot read private draft for failure counts")
					}
				}
			} else if !section.QualityApproved {
				report.Error = "section returned without quality approval"
			}
			if generationErr != nil && lastSourceAttempt > 0 {
				name := fmt.Sprintf("private-target-%.0f-%s-source-%02d.json", band, spec.key, lastSourceAttempt)
				data, readErr := os.ReadFile(filepath.Join(output, name))
				var source mockExamSource
				if readErr == nil && json.Unmarshal(data, &source) == nil {
					content.Passage, content.AudioScript = source.Passage, source.AudioScript
					report.CountsSource = "captured_source_and_questions"
				} else {
					t.Error("cannot read private source for failure counts")
				}
			}
			report.WordCount = mockExamWordCount(content.Passage + content.AudioScript)
			report.QuestionCount = len(content.Questions)
			report.WritingTaskCount = len(content.WritingPrompts)
			for _, prompt := range content.WritingPrompts {
				report.WritingPromptWordCount += mockExamWordCount(prompt.Instructions)
			}
			report.CaptureFailed = captureFailed
			data, encodeErr := json.MarshalIndent(report, "", "  ")
			if encodeErr != nil || os.WriteFile(filepath.Join(output, spec.key+"-report.json"), data, 0600) != nil {
				t.Error("cannot save private section report")
			}
			t.Logf("section=%s model=%s elapsed_seconds=%.1f words=%d questions=%d writing_tasks=%d writing_prompt_words=%d counts_source=%s approved=%t",
				spec.key, report.Model, elapsed, report.WordCount, report.QuestionCount,
				report.WritingTaskCount, report.WritingPromptWordCount, report.CountsSource, report.Approved)
			if captureFailed {
				t.Error("one or more private capture artifacts could not be saved")
			}
			if report.Error != "" {
				t.Error(report.Error)
			}
		})
	}
}

type mockExamSectionLiveReport struct {
	Model                  string                 `json:"model"`
	ConfigurationSource    string                 `json:"configurationSource"`
	SectionKey             string                 `json:"sectionKey"`
	TargetBand             float64                `json:"targetBand"`
	Approved               bool                   `json:"approved"`
	Error                  string                 `json:"error,omitempty"`
	ElapsedSeconds         float64                `json:"elapsedSeconds"`
	CountsSource           string                 `json:"countsSource"`
	WordCount              int                    `json:"wordCount"`
	QuestionCount          int                    `json:"questionCount"`
	WritingTaskCount       int                    `json:"writingTaskCount"`
	WritingPromptWordCount int                    `json:"writingPromptWordCount"`
	CaptureFailed          bool                   `json:"captureFailed"`
	Section                domain.MockExamSection `json:"section"`
}
