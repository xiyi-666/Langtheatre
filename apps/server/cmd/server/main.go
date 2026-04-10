package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/cache"
	"github.com/linguaquest/server/internal/config"
	"github.com/linguaquest/server/internal/graph"
	"github.com/linguaquest/server/internal/health"
	httpserver "github.com/linguaquest/server/internal/http"
	"github.com/linguaquest/server/internal/migrate"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
)

func main() {
	cfg := config.Load()
	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			EnableTracing:    true,
			TracesSampleRate: 0.1,
		}); err != nil {
			log.Printf("sentry init failed: %v", err)
		} else {
			defer sentry.Flush(2 * time.Second)
		}
	}
	var dataStore service.Store
	var pgProber health.Prober
	var cleanupFns []func()
	if cfg.DatabaseURL != "" {
		if err := migrate.ApplyMigrations(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
			log.Fatalf("auto migration failed: %v", err)
		}
		log.Printf("auto migration completed from %s", cfg.MigrationsDir)
		pgStore, err := store.NewPostgresStore(cfg.DatabaseURL)
		if err != nil {
			log.Printf("postgres init failed, fallback to memory store: %v", err)
			dataStore = store.NewMemoryStore()
		} else {
			cleanupFns = append(cleanupFns, pgStore.Close)
			dataStore = pgStore
			pgProber = pgStore
			log.Printf("using PostgreSQL store")
		}
	} else if cfg.SQLitePath != "" {
		sqliteStore, err := store.NewSQLiteStore(cfg.SQLitePath)
		if err != nil {
			log.Printf("sqlite init failed, fallback to memory store: %v", err)
			dataStore = store.NewMemoryStore()
		} else {
			cleanupFns = append(cleanupFns, func() { _ = sqliteStore.Close() })
			dataStore = sqliteStore
			log.Printf("using SQLite store at %s", cfg.SQLitePath)
		}
	} else {
		dataStore = store.NewMemoryStore()
		log.Printf("DATABASE_URL/SUPABASE_DB_URL not set, using memory store")
	}
	defer func() {
		for _, fn := range cleanupFns {
			fn()
		}
	}()
	redisClient := cache.New(cfg.RedisAddr)
	generator := ai.NewOpenAIGenerator(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIBaseURL)
	tts := ai.NewAPITTS(
		cfg.TTSAPIURL,
		cfg.TTSAPIKey,
		cfg.TTSVoice,
		cfg.TTSUseUploadPrompt,
		cfg.TTSPromptAudioPath,
		cfg.TTSReturnJSON,
		cfg.TTSTimeoutSeconds,
		cfg.TTSMaxRetries,
	)
	svc := service.New(dataStore, redisClient, generator, tts, cfg.JWTSecret)
	schema, err := graph.NewSchema(svc)
	if err != nil {
		log.Fatalf("failed to build schema: %v", err)
	}
	checker := health.Checker{
		Postgres: pgProber,
		Redis:    redisClient,
		Timeout:  2 * time.Second,
	}
	mux := httpserver.NewMux(schema, cfg.JWTSecret, func(ctx context.Context) httpserver.HealthResult {
		result := checker.Check(ctx)
		return httpserver.HealthResult{
			OK:        result.OK,
			Timestamp: result.Timestamp,
			Checks:    result.Checks,
		}
	})
	log.Printf("LinguaQuest API listening on :%s", cfg.Port)
	if err = http.ListenAndServe(":"+cfg.Port, httpserver.WrapWithBaseMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
