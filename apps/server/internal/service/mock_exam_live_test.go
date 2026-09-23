package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/domain"
)

// 显式开启的付费集成测试；只读本地配置，不改数据库或调用出题模型。
// IELTS_AUDIO_PAPER 可指定已通过真实审核的完整私有试卷，验收全部四段音频。
// 这里只检验音频生成、解码和时长，不能代替人工听辨或试卷审核。
func TestLiveMockListeningAudio(t *testing.T) {
	if os.Getenv("IELTS_AUDIO_LIVE") != "1" {
		t.Skip("set IELTS_AUDIO_LIVE=1 and explicit draft/output paths for paid TTS validation")
	}
	draftPath, output := os.Getenv("IELTS_AUDIO_DRAFT"), os.Getenv("IELTS_AUDIO_OUTPUT_DIR")
	paperPath := os.Getenv("IELTS_AUDIO_PAPER")
	if (draftPath == "" && paperPath == "") || (draftPath != "" && paperPath != "") || !filepath.IsAbs(output) {
		t.Fatal("explicit draft path and absolute private output directory are required")
	}
	probe := os.Getenv("IELTS_AUDIO_FFPROBE")
	if probe == "" {
		var err error
		probe, err = exec.LookPath("ffprobe")
		if err != nil {
			t.Fatal("ffprobe is required before making paid requests")
		}
	}
	decoder, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("ffmpeg is required for full decode validation before paid requests")
	}
	inputPath := draftPath
	if paperPath != "" {
		inputPath = paperPath
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal("cannot read private listening draft")
	}
	var sections []domain.MockExamSection
	if paperPath != "" {
		var paper domain.MockExam
		if json.Unmarshal(data, &paper) != nil || ai.ValidateMockExamPaper(paper) != nil {
			t.Fatal("paper must be complete and reviewed before paid audio validation")
		}
		for _, section := range paper.Sections {
			if section.Skill == "LISTENING" {
				sections = append(sections, section)
			}
		}
	} else {
		var draft struct{ Title, AudioScript string }
		if json.Unmarshal(data, &draft) != nil || draft.AudioScript == "" {
			t.Fatal("draft must contain an audioScript")
		}
		sections = append(sections, domain.MockExamSection{Key: "LISTENING_1", Title: draft.Title, AudioScript: draft.AudioScript})
	}
	var turns []domain.Dialogue
	var sectionKeys []string
	for _, section := range sections {
		partTurns, parseErr := mockListeningTurns(section.AudioScript)
		if parseErr != nil || len(partTurns) > 40 {
			t.Fatal("each listening section requires 1..40 valid TTS turns")
		}
		turns = append(turns, partTurns...)
		for range partTurns {
			sectionKeys = append(sectionKeys, section.Key)
		}
	}
	serverDir, _ := filepath.Abs(filepath.Join("..", ".."))
	env, err := godotenv.Read(filepath.Join(serverDir, ".env"))
	if err != nil {
		t.Fatal("cannot read local server configuration")
	}
	for _, key := range []string{"DATABASE_URL", "SUPABASE_DB_URL", "SUPBASE_DB_URL"} {
		if env[key] != "" {
			t.Fatal("live TTS harness currently requires local SQLite configuration")
		}
	}
	dbPath := env["SQLITE_PATH"]
	if dbPath == "" {
		t.Fatal("local SQLite configuration is required")
	}
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(serverDir, dbPath)
	}
	uriPath := filepath.ToSlash(dbPath)
	if filepath.VolumeName(dbPath) != "" {
		uriPath = "/" + uriPath
	}
	dsn := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		t.Fatal("cannot open local configuration read-only")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var config domain.TTSConfig
	err = db.QueryRowContext(ctx, "SELECT provider,model,base_url,api_key,voice,audio_format FROM tts_configs WHERE id=1").Scan(
		&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.Voice, &config.AudioFormat,
	)
	if err != nil || config.APIKey == "" {
		t.Fatal("persisted TTS configuration is missing or unreadable")
	}
	tts := ai.NewAPITTS(config.Provider, config.BaseURL, config.APIKey, config.Voice, config.Model, config.AudioFormat, false, "", true, 180, 1)
	svc := &Service{tts: tts, ttsSlots: make(chan struct{}, 1), mediaDir: output}
	started := time.Now()
	var urls []string
	for _, section := range sections {
		partURLs, audioErr := svc.mockListeningAudio(ctx, fmt.Sprintf("live-%d", started.Unix()), section)
		if audioErr != nil {
			t.Fatal("live TTS failed; no valid audio report produced (provider body omitted)")
		}
		urls = append(urls, partURLs...)
		t.Logf("section=%s generated_audio_segments=%d", section.Key, len(partURLs))
	}
	if len(urls) != len(turns) {
		t.Fatal("audio segment count does not match speaker turns")
	}
	type segment struct {
		Section string  `json:"section"`
		Speaker string  `json:"speaker"`
		Path    string  `json:"path"`
		Words   int     `json:"words"`
		Seconds float64 `json:"seconds"`
	}
	segments := make([]segment, 0, len(urls))
	totalSeconds, totalWords := 0.0, 0
	for i, audioURL := range urls {
		if !strings.HasPrefix(audioURL, "/media/tts/") {
			t.Fatal("TTS returned a remote URL; local decode validation is not available")
		}
		path := filepath.Join(output, filepath.FromSlash(strings.TrimPrefix(audioURL, "/media/")))
		info, err := os.Stat(path)
		if err != nil || info.Size() < 1024 {
			t.Fatalf("audio segment %d is empty or missing", i+1)
		}
		seconds, decodeErr := mockLiveDecodedDuration(ctx, probe, decoder, path)
		if decodeErr != nil {
			t.Fatalf("audio segment %d is not decodable", i+1)
		}
		words := len(strings.Fields(turns[i].Text))
		totalWords += words
		totalSeconds += seconds
		segments = append(segments, segment{sectionKeys[i], turns[i].Speaker, path, words, seconds})
		if rate := float64(words) * 60 / seconds; rate < 70 || rate > 260 {
			t.Errorf("audio segment %d has suspicious rate %.1f words/minute", i+1, rate)
		}
	}
	wpm := float64(totalWords) * 60 / totalSeconds
	report := struct {
		Provider               string    `json:"provider"`
		Model                  string    `json:"model"`
		Segments               []segment `json:"segments"`
		Seconds                float64   `json:"seconds"`
		WordsPerMinute         float64   `json:"wordsPerMinute"`
		ElapsedSeconds         float64   `json:"elapsedSeconds"`
		HumanListeningReviewed bool      `json:"humanListeningReviewed"`
	}{config.Provider, config.Model, segments, totalSeconds, wpm, time.Since(started).Seconds(), false}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(filepath.Join(output, "audio-report.json"), encoded, 0600); err != nil {
		t.Fatal("cannot save private audio report")
	}
	t.Logf("provider=%s model=%s turns=%d words=%d seconds=%.1f words_per_minute=%.1f elapsed_seconds=%.1f", config.Provider, config.Model, len(turns), totalWords, totalSeconds, wpm, time.Since(started).Seconds())
	if wpm < 90 || wpm > 220 {
		t.Error("audio duration is outside the broad speech-rate sanity range; investigate omissions or excessive pauses")
	}
	for _, section := range sections {
		seconds, words := 0.0, 0
		for _, item := range segments {
			if item.Section == section.Key {
				seconds += item.Seconds
				words += item.Words
			}
		}
		rate := float64(words) * 60 / seconds
		t.Logf("section=%s words=%d seconds=%.1f words_per_minute=%.1f", section.Key, words, seconds, rate)
		if rate < 90 || rate > 220 {
			t.Errorf("section %s speech rate needs investigation", section.Key)
		}
	}
}

func mockLiveDecodedDuration(ctx context.Context, probe, decoder, path string) (float64, error) {
	// 强制选择音轨并完整解码；仅 ffprobe 元数据/时长不能排除损坏载荷。
	if output, err := exec.CommandContext(ctx, decoder, "-nostdin", "-v", "error", "-xerror", "-i", path, "-map", "0:a:0", "-f", "null", "-").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "" {
		return 0, errors.New("audio stream missing or complete decode failed")
	}
	result, err := exec.CommandContext(ctx, probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	seconds, parseErr := strconv.ParseFloat(strings.TrimSpace(string(result)), 64)
	if err != nil || parseErr != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0, errors.New("invalid audio duration")
	}
	return seconds, nil
}

func TestMockExamAudioDecodeGate(t *testing.T) {
	decoder, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("offline audio decoder regression requires ffmpeg")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("offline audio decoder regression requires ffprobe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.m4a")
	if err := exec.CommandContext(ctx, decoder, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac", "-movflags", "+faststart", valid).Run(); err != nil {
		t.Fatal("cannot create offline AAC test signal")
	}
	if _, err := mockLiveDecodedDuration(ctx, probe, decoder, valid); err != nil {
		t.Fatal("valid audio rejected")
	}
	data, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(data, []byte("mdat"))
	if start < 0 {
		t.Fatal("test container has no media payload")
	}
	for i := start + 4; i < len(data); i++ {
		data[i] = 0
	}
	corrupt := filepath.Join(dir, "corrupt.m4a")
	if err := os.WriteFile(corrupt, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := mockLiveDecodedDuration(ctx, probe, decoder, corrupt); err == nil {
		t.Fatal("corrupt payload passed audio validation")
	}
	video := filepath.Join(dir, "no-audio.mp4")
	if err := exec.CommandContext(ctx, decoder, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=black:size=32x32:duration=1", "-an", "-c:v", "mpeg4", video).Run(); err != nil {
		t.Fatal("cannot create offline video-only fixture")
	}
	if _, err := mockLiveDecodedDuration(ctx, probe, decoder, video); err == nil {
		t.Fatal("video-only file passed audio validation")
	}
}
