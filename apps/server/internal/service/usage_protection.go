package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// UsageProtectionOptions applies fair-use controls to the free public beta.
// It is intentionally only enabled for the mini-program edition by main.go.
type UsageProtectionOptions struct {
	Enabled        bool
	Cooldown       time.Duration
	MaxActiveTasks int
}

type aiRequestGuard struct {
	mu             sync.Mutex
	cooldown       time.Duration
	maxActiveTasks int
	nextAllowed    map[string]time.Time
	activeTasks    map[string]int
}

func newAIRequestGuard(options UsageProtectionOptions) *aiRequestGuard {
	if !options.Enabled {
		return nil
	}
	if options.Cooldown <= 0 {
		options.Cooldown = 12 * time.Second
	}
	if options.MaxActiveTasks <= 0 {
		options.MaxActiveTasks = 2
	}
	return &aiRequestGuard{
		cooldown:       options.Cooldown,
		maxActiveTasks: options.MaxActiveTasks,
		nextAllowed:    map[string]time.Time{},
		activeTasks:    map[string]int{},
	}
}

func (g *aiRequestGuard) reserve(userID string) (func(), error) {
	if g == nil {
		return func() {}, nil
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.activeTasks[userID] >= g.maxActiveTasks {
		return nil, fmt.Errorf("产品处于内测：每个账号同时最多可处理 %d 个 AI 任务，请等待当前任务完成。", g.maxActiveTasks)
	}
	if next := g.nextAllowed[userID]; next.After(now) {
		waitSeconds := int(next.Sub(now).Seconds()) + 1
		return nil, fmt.Errorf("请求过于频繁，请在 %d 秒后再试", waitSeconds)
	}
	g.activeTasks[userID]++
	g.nextAllowed[userID] = now.Add(g.cooldown)
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			if g.activeTasks[userID] <= 1 {
				delete(g.activeTasks, userID)
				return
			}
			g.activeTasks[userID]--
		})
	}, nil
}

func (s *Service) reserveAIRequest(userID string) (func(), error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("unauthorized")
	}
	return s.usageGuard.reserve(userID)
}
