package health

import (
	"context"
	"time"
)

type Prober interface {
	Ping(ctx context.Context) error
}

type Checker struct {
	Postgres Prober
	Redis    Prober
	Timeout  time.Duration
}

type Result struct {
	OK        bool              `json:"ok"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

func (c Checker) Check(ctx context.Context) Result {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	result := Result{
		OK:        true,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    map[string]string{},
	}

	if c.Postgres != nil {
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		err := c.Postgres.Ping(checkCtx)
		cancel()
		if err != nil {
			result.OK = false
			result.Checks["postgres"] = "down: " + err.Error()
		} else {
			result.Checks["postgres"] = "up"
		}
	} else {
		result.Checks["postgres"] = "not_configured"
	}

	if c.Redis != nil {
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		err := c.Redis.Ping(checkCtx)
		cancel()
		if err != nil {
			result.OK = false
			result.Checks["redis"] = "down: " + err.Error()
		} else {
			result.Checks["redis"] = "up"
		}
	} else {
		result.Checks["redis"] = "not_configured"
	}

	return result
}
