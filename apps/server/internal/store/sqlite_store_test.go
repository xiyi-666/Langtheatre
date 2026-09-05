package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

func TestProductionCompatibilityMigrationContract(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "010_production_compatibility_sync.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"CREATE OR REPLACE FUNCTION sync_writing_prompt_minutes()",
		"DROP TRIGGER IF EXISTS trg_sync_writing_prompt_minutes ON writing_sessions",
		"BEFORE INSERT OR UPDATE ON writing_sessions",
		"CEIL(NEW.time_limit_seconds / 60.0)",
		"jsonb_typeof(NEW.prompt -> 'Instructions') IS DISTINCT FROM 'string'",
		"You should spend about [0-9]+ minutes on this task\\.",
		"WHERE status = 'READY'",
		"AND audio_status = 'PENDING'",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("migration missing contract fragment %q", expected)
		}
	}
	if strings.Contains(sql, "DATABASE_URL") || strings.Contains(sql, "SUPABASE_DB_URL") {
		t.Fatal("migration must not contain connection configuration")
	}
	if strings.Contains(sql, "jsonb_typeof(NEW.prompt -> 'Instructions') <> 'string'") {
		t.Fatal("migration uses NULL-unsafe Instructions type guard")
	}
}

func TestPostgresTheaterProjectionContracts(t *testing.T) {
	raw, err := os.ReadFile("postgres_store.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	listSource := sourceBetween(t, source, "func (s *PostgresStore) ListTheatersByUser", "func (s *PostgresStore) SetTheaterFavorite")
	for _, heavy := range []string{"COALESCE(characters", "dialogues,", "quiz_questions"} {
		if strings.Contains(listSource, heavy) {
			t.Fatalf("ListTheatersByUser still selects heavy field marker %q", heavy)
		}
	}
	sharedSource := sourceBetween(t, source, "func (s *PostgresStore) GetTheaterByShareCode", "func (s *PostgresStore) ListTheatersByUser")
	for _, detail := range []string{"COALESCE(characters", "dialogues,", "quiz_questions"} {
		if !strings.Contains(sharedSource, detail) {
			t.Fatalf("GetTheaterByShareCode missing detail field marker %q", detail)
		}
	}
}

func sourceBetween(t *testing.T, source string, start string, end string) string {
	t.Helper()
	startIndex := strings.Index(source, start)
	endIndex := strings.Index(source, end)
	if startIndex < 0 || endIndex <= startIndex {
		t.Fatalf("source markers not found: start=%q end=%q", start, end)
	}
	return source[startIndex:endIndex]
}

func TestSQLiteStoreMigratesLegacyUniqueEmailUsers(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	_, err = legacyDB.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		nickname TEXT,
		avatar_url TEXT,
		bio TEXT,
		total_xp INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		legacyDB.Close()
		t.Fatalf("create legacy users table: %v", err)
	}
	_, err = legacyDB.Exec(
		`INSERT INTO users (id, email, password_hash, nickname, avatar_url, bio, total_xp, created_at) VALUES (?, ?, ?, '', '', '', 0, ?)`,
		"12345678-1234-1234-1234-123456789012",
		"shared@linguaquest.app",
		"legacy-hash",
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if closeErr := legacyDB.Close(); closeErr != nil {
		t.Fatalf("close legacy database: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}

	store, err := NewSQLiteStore(databasePath)
	if err != nil {
		t.Fatalf("migrate legacy SQLite database: %v", err)
	}
	defer store.Close()

	legacy, err := store.GetUserByID("12345678-1234-1234-1234-123456789012")
	if err != nil {
		t.Fatalf("load legacy user: %v", err)
	}
	if !strings.HasPrefix(legacy.Username, "legacy_") || !legacy.EmailVerified {
		t.Fatalf("legacy authentication fields not migrated: %+v", legacy)
	}
	for _, username := range []string{"second_user", "third_user"} {
		if _, err = store.CreateUser(username, "shared@linguaquest.app", "hash", true); err != nil {
			t.Fatalf("create %s with shared email: %v", username, err)
		}
	}
	if _, err = store.CreateUser("fourth_user", "shared@linguaquest.app", "hash", true); err == nil {
		t.Fatal("expected fourth account with the same email to be rejected")
	}
}

func TestSQLiteReadingMaterialMetadataRoundTrip(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "reading.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	material := domain.ReadingMaterial{
		ID: "11111111-1111-1111-1111-111111111111", UserID: "22222222-2222-2222-2222-222222222222",
		Exam: "IELTS", Language: "ENGLISH", Level: "advanced", Topic: "urban policy",
		Band: 7.3, Stage: "Stage 9", Section: "Section 3", SkillFocus: "author stance",
		QuestionType: "Summary Completion", ScenarioFamily: "urban governance", Title: "Urban policy",
		Questions:   []domain.QuizQuestion{{Type: "Summary Completion", Question: "Complete the summary", Options: []string{"policy"}, AnswerKey: "policy", SummaryText: "A ___ response", WordBank: []string{"policy"}, Answers: []string{"policy"}}},
		AudioStatus: "PENDING", Status: "GENERATING", CreatedAt: time.Now().UTC(),
	}
	if _, err = store.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Band != material.Band || got.Stage != material.Stage || got.Section != material.Section || got.SkillFocus != material.SkillFocus || got.QuestionType != material.QuestionType || got.ScenarioFamily != material.ScenarioFamily {
		t.Fatalf("metadata round trip = %+v", got)
	}
	if len(got.Questions) != 1 || got.Questions[0].SummaryText != "A ___ response" || len(got.Questions[0].Answers) != 1 {
		t.Fatalf("extended question fields lost: %+v", got.Questions)
	}
	got.QuestionType = "TFNG"
	got.ScenarioFamily = "coastal adaptation"
	if _, err = store.UpdateReadingMaterialExisting(got); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListReadingMaterialsByUser(material.UserID, "IELTS")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].QuestionType != "TFNG" || items[0].ScenarioFamily != "coastal adaptation" {
		t.Fatalf("updated list round trip = %+v", items)
	}
}

func TestSQLiteStoreMigratesLegacyReadingMaterialMetadata(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy-reading.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	_, err = legacyDB.Exec(`CREATE TABLE reading_materials (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		exam TEXT NOT NULL,
		language TEXT NOT NULL,
		level TEXT NOT NULL,
		topic TEXT NOT NULL,
		title TEXT NOT NULL,
		passage TEXT NOT NULL,
		vocabulary TEXT NOT NULL DEFAULT '[]',
		questions TEXT NOT NULL DEFAULT '[]',
		source_ids TEXT NOT NULL DEFAULT '[]',
		generation_note TEXT NOT NULL DEFAULT '',
		audio_url TEXT NOT NULL DEFAULT '',
		audio_urls TEXT NOT NULL DEFAULT '[]',
		audio_status TEXT NOT NULL DEFAULT 'PENDING',
		vocabulary_items TEXT NOT NULL DEFAULT '[]',
		association_sentences TEXT NOT NULL DEFAULT '[]',
		grammar_insights TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		legacyDB.Close()
		t.Fatalf("create legacy reading_materials table: %v", err)
	}
	materialID := "33333333-3333-3333-3333-333333333333"
	userID := "44444444-4444-4444-4444-444444444444"
	_, err = legacyDB.Exec(`INSERT INTO reading_materials (
		id, user_id, exam, language, level, topic, title, passage, created_at
	) VALUES (?, ?, 'IELTS', 'ENGLISH', 'intermediate', 'legacy topic', 'Legacy reading', 'Legacy passage', ?)`,
		materialID, userID, time.Now().UTC().Format(time.RFC3339Nano))
	if closeErr := legacyDB.Close(); closeErr != nil {
		t.Fatalf("close legacy database: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("insert legacy reading material: %v", err)
	}

	store, err := NewSQLiteStore(databasePath)
	if err != nil {
		t.Fatalf("migrate legacy reading database: %v", err)
	}

	got, err := store.GetReadingMaterial(materialID, userID)
	if err != nil {
		t.Fatalf("query migrated reading material: %v", err)
	}
	if got.Band != 0 || got.Stage != "" || got.Section != "" || got.SkillFocus != "" || got.QuestionType != "" || got.ScenarioFamily != "" {
		t.Fatalf("unexpected migrated metadata defaults: %+v", got)
	}
	if got.Status != "READY" || got.GenerationProgress != 100 {
		t.Fatalf("legacy reading progress fields not migrated: %+v", got)
	}
	if _, err = store.ListReadingMaterialsByUser(userID, "IELTS"); err != nil {
		t.Fatalf("list migrated reading materials: %v", err)
	}

	// Re-opening the same database verifies the ALTER path is idempotent.
	if err = store.Close(); err != nil {
		t.Fatalf("close migrated store: %v", err)
	}
	store, err = NewSQLiteStore(databasePath)
	if err != nil {
		t.Fatalf("re-open migrated reading database: %v", err)
	}
	defer store.Close()
	if _, err = store.GetReadingMaterial(materialID, userID); err != nil {
		t.Fatalf("query after idempotent migration: %v", err)
	}
}
