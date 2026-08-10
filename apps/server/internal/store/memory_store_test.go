package store

import (
	"testing"

	"github.com/linguaquest/server/internal/domain"
)

func TestCreateUser(t *testing.T) {
	s := NewMemoryStore()
	user, err := s.CreateUser("alice@example.com", "hash123")
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q", user.Email)
	}
	if user.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	s := NewMemoryStore()
	s.CreateUser("alice@example.com", "hash")
	_, err := s.CreateUser("alice@example.com", "hash2")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestGetUserByEmail(t *testing.T) {
	s := NewMemoryStore()
	created, _ := s.CreateUser("bob@example.com", "hash")
	found, err := s.GetUserByEmail("bob@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail error: %v", err)
	}
	if found.ID != created.ID {
		t.Error("user IDs don't match")
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetUserByEmail("nobody@example.com")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestGetUserByID(t *testing.T) {
	s := NewMemoryStore()
	created, _ := s.CreateUser("c@example.com", "h")
	found, err := s.GetUserByID(created.ID)
	if err != nil {
		t.Fatalf("GetUserByID error: %v", err)
	}
	if found.Email != "c@example.com" {
		t.Error("email mismatch")
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetUserByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestUpdateUserProfile(t *testing.T) {
	s := NewMemoryStore()
	user, _ := s.CreateUser("d@e.com", "h")
	updated, err := s.UpdateUserProfile(user.ID, "Dave", "https://avatar.url", "Hello")
	if err != nil {
		t.Fatalf("UpdateUserProfile error: %v", err)
	}
	if updated.Nickname != "Dave" {
		t.Errorf("Nickname = %q", updated.Nickname)
	}
	if updated.AvatarURL != "https://avatar.url" {
		t.Errorf("AvatarURL = %q", updated.AvatarURL)
	}
	if updated.Bio != "Hello" {
		t.Errorf("Bio = %q", updated.Bio)
	}
}

func TestUpdateUserProfile_NotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.UpdateUserProfile("bad-id", "x", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateUserProfile_EmptyNicknameKeepsExisting(t *testing.T) {
	s := NewMemoryStore()
	user, _ := s.CreateUser("e@f.com", "h")
	s.UpdateUserProfile(user.ID, "Original", "", "")
	updated, _ := s.UpdateUserProfile(user.ID, "", "url", "bio")
	if updated.Nickname != "Original" {
		t.Errorf("Nickname should remain 'Original', got %q", updated.Nickname)
	}
}

func TestAddUserXP(t *testing.T) {
	s := NewMemoryStore()
	user, _ := s.CreateUser("xp@test.com", "h")
	if err := s.AddUserXP(user.ID, 50); err != nil {
		t.Fatalf("AddUserXP error: %v", err)
	}
	found, _ := s.GetUserByID(user.ID)
	if found.TotalXP != 50 {
		t.Errorf("TotalXP = %d, want 50", found.TotalXP)
	}
	s.AddUserXP(user.ID, 30)
	found, _ = s.GetUserByID(user.ID)
	if found.TotalXP != 80 {
		t.Errorf("TotalXP = %d, want 80", found.TotalXP)
	}
}

func TestAddUserXP_NotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.AddUserXP("missing", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSaveAndGetTheater(t *testing.T) {
	s := NewMemoryStore()
	theater := domain.Theater{ID: "t1", UserID: "u1", Language: "ENGLISH", Topic: "greetings"}
	saved, err := s.SaveTheater(theater)
	if err != nil {
		t.Fatalf("SaveTheater error: %v", err)
	}
	if saved.ID != "t1" {
		t.Error("ID mismatch")
	}
	got, err := s.GetTheater("t1")
	if err != nil {
		t.Fatalf("GetTheater error: %v", err)
	}
	if got.Topic != "greetings" {
		t.Errorf("Topic = %q", got.Topic)
	}
}

func TestGetTheater_NotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetTheater("missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetTheaterFavorite(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1"})
	err := s.SetTheaterFavorite("u1", "t1", true)
	if err != nil {
		t.Fatalf("SetTheaterFavorite error: %v", err)
	}
	got, _ := s.GetTheater("t1")
	if !got.IsFavorite {
		t.Error("IsFavorite should be true")
	}
}

func TestSetTheaterFavorite_WrongUser(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1"})
	err := s.SetTheaterFavorite("u2", "t1", true)
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestSetTheaterShareCode(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1"})
	err := s.SetTheaterShareCode("u1", "t1", "ABC123")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	got, _ := s.GetTheater("t1")
	if got.ShareCode != "ABC123" {
		t.Errorf("ShareCode = %q", got.ShareCode)
	}
}

func TestGetTheaterByShareCode(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1", ShareCode: "XYZ"})
	got, err := s.GetTheaterByShareCode("xyz")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.ID != "t1" {
		t.Errorf("ID = %q", got.ID)
	}
}

func TestGetTheaterByShareCode_NotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetTheaterByShareCode("MISSING")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListTheatersByUser(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1", Language: "ENGLISH", Status: "done"})
	s.SaveTheater(domain.Theater{ID: "t2", UserID: "u1", Language: "CANTONESE", Status: "done"})
	s.SaveTheater(domain.Theater{ID: "t3", UserID: "u2", Language: "ENGLISH", Status: "done"})

	list, err := s.ListTheatersByUser("u1", "", "", nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 theaters, got %d", len(list))
	}

	list2, _ := s.ListTheatersByUser("u1", "ENGLISH", "", nil)
	if len(list2) != 1 {
		t.Errorf("expected 1 English theater, got %d", len(list2))
	}
}

func TestListTheatersByUser_FavoriteFilter(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1", IsFavorite: true})
	s.SaveTheater(domain.Theater{ID: "t2", UserID: "u1", IsFavorite: false})

	fav := true
	list, _ := s.ListTheatersByUser("u1", "", "", &fav)
	if len(list) != 1 {
		t.Errorf("expected 1 favorite, got %d", len(list))
	}
}

func TestDeleteTheater(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1"})
	err := s.DeleteTheater("u1", "t1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	_, err = s.GetTheater("t1")
	if err == nil {
		t.Fatal("theater should be deleted")
	}
}

func TestDeleteTheater_WrongUser(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1"})
	err := s.DeleteTheater("u2", "t1")
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestDeleteTheater_AlsoDeletesRoleplaySessions(t *testing.T) {
	s := NewMemoryStore()
	s.SaveTheater(domain.Theater{ID: "t1", UserID: "u1"})
	s.CreateRoleplaySession(domain.RoleplaySession{ID: "rs1", UserID: "u1", TheaterID: "t1"})
	s.DeleteTheater("u1", "t1")
	_, err := s.GetRoleplaySession("rs1", "u1")
	if err == nil {
		t.Fatal("roleplay session should be deleted with theater")
	}
}

func TestModelConfig_SaveAndGet(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetModelConfig()
	if err == nil {
		t.Fatal("expected error when no config set")
	}
	cfg := domain.ModelConfig{Provider: "openai", Model: "gpt-4", BaseURL: "https://api.openai.com", APIKey: "key"}
	s.SaveModelConfig(cfg)
	got, err := s.GetModelConfig()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Provider != "openai" {
		t.Errorf("Provider = %q", got.Provider)
	}
}

func TestTTSConfig_SaveAndGet(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetTTSConfig()
	if err == nil {
		t.Fatal("expected error when no TTS config set")
	}
	cfg := domain.TTSConfig{Provider: "XIAOMI", BaseURL: "https://example.com", APIKey: "key"}
	s.SaveTTSConfig(cfg)
	got, err := s.GetTTSConfig()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Provider != "XIAOMI" {
		t.Errorf("Provider = %q", got.Provider)
	}
}

func TestSaveAndGetReadingMaterial(t *testing.T) {
	s := NewMemoryStore()
	mat := domain.ReadingMaterial{ID: "r1", UserID: "u1", Title: "Test Reading"}
	s.SaveReadingMaterial(mat)
	got, err := s.GetReadingMaterial("r1", "u1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Title != "Test Reading" {
		t.Errorf("Title = %q", got.Title)
	}
}

func TestGetReadingMaterial_WrongUser(t *testing.T) {
	s := NewMemoryStore()
	s.SaveReadingMaterial(domain.ReadingMaterial{ID: "r1", UserID: "u1"})
	_, err := s.GetReadingMaterial("r1", "u2")
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestListReadingMaterialsByUser(t *testing.T) {
	s := NewMemoryStore()
	s.SaveReadingMaterial(domain.ReadingMaterial{ID: "r1", UserID: "u1", Exam: "IELTS"})
	s.SaveReadingMaterial(domain.ReadingMaterial{ID: "r2", UserID: "u1", Exam: "TOEFL"})
	s.SaveReadingMaterial(domain.ReadingMaterial{ID: "r3", UserID: "u2", Exam: "IELTS"})

	list, _ := s.ListReadingMaterialsByUser("u1", "")
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
	list2, _ := s.ListReadingMaterialsByUser("u1", "IELTS")
	if len(list2) != 1 {
		t.Errorf("expected 1 IELTS, got %d", len(list2))
	}
}

func TestRoleplaySession_CRUD(t *testing.T) {
	s := NewMemoryStore()
	session := domain.RoleplaySession{ID: "s1", UserID: "u1", TheaterID: "t1", Status: "active"}
	created, err := s.CreateRoleplaySession(session)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if created.Status != "active" {
		t.Errorf("Status = %q", created.Status)
	}

	got, err := s.GetRoleplaySession("s1", "u1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.TheaterID != "t1" {
		t.Errorf("TheaterID = %q", got.TheaterID)
	}

	got.Status = "completed"
	updated, err := s.UpdateRoleplaySession(got)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if updated.Status != "completed" {
		t.Errorf("Status = %q after update", updated.Status)
	}
}

func TestGetRoleplaySession_WrongUser(t *testing.T) {
	s := NewMemoryStore()
	s.CreateRoleplaySession(domain.RoleplaySession{ID: "s1", UserID: "u1"})
	_, err := s.GetRoleplaySession("s1", "u2")
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestListCourses(t *testing.T) {
	s := NewMemoryStore()
	all, err := s.ListCourses("")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 courses, got %d", len(all))
	}
	cantonese, _ := s.ListCourses("CANTONESE")
	if len(cantonese) != 1 {
		t.Errorf("expected 1 Cantonese course, got %d", len(cantonese))
	}
	english, _ := s.ListCourses("ENGLISH")
	if len(english) != 1 {
		t.Errorf("expected 1 English course, got %d", len(english))
	}
}

func TestSavePracticeRecord(t *testing.T) {
	s := NewMemoryStore()
	err := s.SavePracticeRecord("u1", "t1", 80, []string{"A", "B"}, 100)
	if err != nil {
		t.Errorf("SavePracticeRecord error: %v", err)
	}
}

func TestSaveReadingPracticeRecord(t *testing.T) {
	s := NewMemoryStore()
	err := s.SaveReadingPracticeRecord("u1", "r1", 90, []string{"C"}, 95)
	if err != nil {
		t.Errorf("SaveReadingPracticeRecord error: %v", err)
	}
}
