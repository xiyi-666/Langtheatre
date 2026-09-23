package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linguaquest/server/internal/questionbank"
)

type bankAudioFailureSynth struct{}

func (bankAudioFailureSynth) Synthesize(context.Context, string, string, string) (string, error) {
	return "", errors.New("PRIVATE_PROVIDER_DIAGNOSTIC")
}

func TestBankAudioFailurePreservesSnapshot(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required for media validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bank.json")
	b := questionbank.Bank{Version: 1, Items: []questionbank.Item{{ID: "one", Part: 1, Question: "What do you enjoy studying?", Source: "fixture.pdf", SourcePage: 1}}}
	raw, _ := json.Marshal(b)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(nil, nil, nil, bankAudioFailureSynth{}, "", Options{MediaDir: dir})
	n, err := s.GenerateSpeakingBankAudio(context.Background(), path, 1)
	if err == nil || n != 0 || strings.Contains(err.Error(), "PRIVATE_PROVIDER_DIAGNOSTIC") {
		t.Fatal(n, err)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != string(raw) {
		t.Fatal("failed synthesis changed bank")
	}
	if _, err = os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("lock not released")
	}
	n, err = s.GenerateSpeakingBankAudio(context.Background(), path, 0)
	if err == nil || n != 0 {
		t.Fatal("unbounded batch accepted")
	}
}

func TestExplicitInvalidBankDoesNotUseDefaultQuestions(t *testing.T) {
	t.Setenv("SPEAKING_BANK_PATH", filepath.Join(t.TempDir(), "missing.json"))
	s := NewWithOptions(nil, nil, nil, nil, "", Options{})
	if _, err := s.StartSpeakingSession("owner"); err == nil {
		t.Fatal("invalid explicit bank silently fell back")
	}
}
