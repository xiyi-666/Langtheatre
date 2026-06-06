package auth

import (
	"testing"
)

const testSecret = "test-secret-key-for-jwt"

func TestCreateAndParseAccessToken(t *testing.T) {
	token, err := CreateAccessToken(testSecret, "user-123", "alice@example.com")
	if err != nil {
		t.Fatalf("CreateAccessToken error: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := ParseAccessToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseAccessToken error: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-123")
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "alice@example.com")
	}
}

func TestParseAccessToken_WrongSecret(t *testing.T) {
	token, err := CreateAccessToken(testSecret, "user-123", "alice@example.com")
	if err != nil {
		t.Fatalf("CreateAccessToken error: %v", err)
	}

	_, err = ParseAccessToken("wrong-secret", token)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestParseAccessToken_Malformed(t *testing.T) {
	_, err := ParseAccessToken(testSecret, "not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

func TestParseAccessToken_EmptyToken(t *testing.T) {
	_, err := ParseAccessToken(testSecret, "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestCreateAccessToken_DifferentUsersProduceDifferentTokens(t *testing.T) {
	t1, _ := CreateAccessToken(testSecret, "user-1", "a@b.com")
	t2, _ := CreateAccessToken(testSecret, "user-2", "c@d.com")
	if t1 == t2 {
		t.Error("tokens for different users should differ")
	}
}
