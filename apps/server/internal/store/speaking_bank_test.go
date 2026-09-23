package store

import (
	"github.com/linguaquest/server/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteSpeakingPreservesBankProvenanceAndAudio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speaking.db")
	db, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	original := domain.SpeakingSession{ID: "bank-session", UserID: "owner", Status: "ACTIVE", Part: 1, CreatedAt: now, UpdatedAt: now,
		Prompts: []domain.SpeakingPrompt{{QuestionID: "bank-item", Part: 1, Question: "What do you enjoy studying?", Source: "fixture.pdf · 第 3 页", AudioURL: "/media/tts/question-bank/test.mp3"}}}
	if _, err = db.CreateSpeakingSession(original); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	restored, err := db.GetSpeakingSession(original.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Prompts) != 1 || restored.Prompts[0] != original.Prompts[0] {
		t.Fatal("bank provenance lost after reopen")
	}
	if _, err = db.GetSpeakingSession(original.ID, "other"); err == nil {
		t.Fatal("foreign session allowed")
	}
}

func TestLatestSpeakingSessionReturnsNewestUnfinishedForCurrentUser(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		store := NewMemoryStore()
		assertLatestSpeakingSelection(t, store)
	})
	t.Run("sqlite", func(t *testing.T) {
		store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "latest-speaking.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		assertLatestSpeakingSelection(t, store)
	})
}

type speakingSessionSelectionStore interface {
	CreateSpeakingSession(domain.SpeakingSession) (domain.SpeakingSession, error)
	LatestSpeakingSession(string) (*domain.SpeakingSession, error)
}

func assertLatestSpeakingSelection(t *testing.T, store speakingSessionSelectionStore) {
	t.Helper()
	base := time.Now().UTC().Add(-time.Hour)
	items := []domain.SpeakingSession{
		{ID: "older-active", UserID: "owner", Status: "ACTIVE", UpdatedAt: base},
		{ID: "completed", UserID: "owner", Status: "COMPLETED", UpdatedAt: base.Add(time.Minute)},
		{ID: "other-user", UserID: "other", Status: "ACTIVE", UpdatedAt: base.Add(2 * time.Minute)},
		{ID: "newer-active", UserID: "owner", Status: "TURN_FAILED", UpdatedAt: base.Add(3 * time.Minute)},
		{ID: "abandoned", UserID: "owner", Status: "ABANDONED", UpdatedAt: base.Add(4 * time.Minute)},
		{ID: "quality-blocked", UserID: "owner", Status: "QUALITY_REVIEW_PENDING", UpdatedAt: base.Add(5 * time.Minute)},
	}
	for index, item := range items {
		created, err := store.CreateSpeakingSession(item)
		if err != nil {
			t.Fatal(err)
		}
		// SQLite owns timestamps on insert, so preserve deterministic ordering there.
		if sqliteStore, ok := store.(*SQLiteStore); ok {
			updated := base.Add(time.Duration(index) * time.Minute).Format(sqliteTimeLayout)
			if _, err = sqliteStore.db.Exec(`UPDATE speaking_sessions SET updated_at=? WHERE id=?`, updated, created.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	latest, err := store.LatestSpeakingSession("owner")
	if err != nil {
		t.Fatal(err)
	}
	if latest == nil || latest.ID != "newer-active" {
		t.Fatalf("latest unfinished session = %+v, want newer-active", latest)
	}
	missing, err := store.LatestSpeakingSession("missing-user")
	if err != nil || missing != nil {
		t.Fatalf("missing user session = %+v, err = %v", missing, err)
	}
}
