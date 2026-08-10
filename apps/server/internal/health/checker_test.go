package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

type okProber struct{}

func (okProber) Ping(_ context.Context) error { return nil }

type failProber struct{ err error }

func (f failProber) Ping(_ context.Context) error { return f.err }

func TestCheck_AllUp(t *testing.T) {
	checker := Checker{
		Postgres: okProber{},
		Redis:    okProber{},
		Timeout:  time.Second,
	}
	result := checker.Check(context.Background())
	if !result.OK {
		t.Error("expected OK = true")
	}
	if result.Checks["postgres"] != "up" {
		t.Errorf("postgres = %q, want up", result.Checks["postgres"])
	}
	if result.Checks["redis"] != "up" {
		t.Errorf("redis = %q, want up", result.Checks["redis"])
	}
	if result.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}

func TestCheck_PostgresDown(t *testing.T) {
	checker := Checker{
		Postgres: failProber{err: errors.New("connection refused")},
		Redis:    okProber{},
		Timeout:  time.Second,
	}
	result := checker.Check(context.Background())
	if result.OK {
		t.Error("expected OK = false")
	}
	if result.Checks["postgres"] != "down: connection refused" {
		t.Errorf("postgres = %q", result.Checks["postgres"])
	}
	if result.Checks["redis"] != "up" {
		t.Errorf("redis = %q", result.Checks["redis"])
	}
}

func TestCheck_RedisDown(t *testing.T) {
	checker := Checker{
		Postgres: okProber{},
		Redis:    failProber{err: errors.New("timeout")},
		Timeout:  time.Second,
	}
	result := checker.Check(context.Background())
	if result.OK {
		t.Error("expected OK = false")
	}
	if result.Checks["redis"] != "down: timeout" {
		t.Errorf("redis = %q", result.Checks["redis"])
	}
}

func TestCheck_BothDown(t *testing.T) {
	checker := Checker{
		Postgres: failProber{err: errors.New("pg err")},
		Redis:    failProber{err: errors.New("redis err")},
		Timeout:  time.Second,
	}
	result := checker.Check(context.Background())
	if result.OK {
		t.Error("expected OK = false")
	}
}

func TestCheck_NilProbers(t *testing.T) {
	checker := Checker{Timeout: time.Second}
	result := checker.Check(context.Background())
	if !result.OK {
		t.Error("expected OK = true when probers are nil")
	}
	if result.Checks["postgres"] != "not_configured" {
		t.Errorf("postgres = %q, want not_configured", result.Checks["postgres"])
	}
	if result.Checks["redis"] != "not_configured" {
		t.Errorf("redis = %q, want not_configured", result.Checks["redis"])
	}
}

func TestCheck_DefaultTimeout(t *testing.T) {
	// Timeout 0 should default to 2 seconds internally (no panic)
	checker := Checker{
		Postgres: okProber{},
		Redis:    okProber{},
	}
	result := checker.Check(context.Background())
	if !result.OK {
		t.Error("expected OK = true")
	}
}
