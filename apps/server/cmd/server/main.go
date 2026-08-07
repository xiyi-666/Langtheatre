package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/analytics"
	"github.com/linguaquest/server/internal/cache"
	"github.com/linguaquest/server/internal/config"
	mailservice "github.com/linguaquest/server/internal/email"
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
	var analyticsReporter *analytics.Reporter
	if cfg.AnalyticsEnabled {
		if analyticsStore, ok := any(dataStore).(analytics.Store); ok {
			analyticsReporter = analytics.NewReporter(analyticsStore, cfg.AnalyticsTimezone)
			defer analyticsReporter.Close()
		} else {
			log.Printf("analytics is unavailable: selected store does not support daily aggregates")
		}
	}
	redisClient := cache.New(cfg.RedisAddr)
	generator := ai.NewOpenAIGenerator(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIBaseURL)
	generator.SetUsageReporter(analyticsReporter)
	if savedModelConfig, err := dataStore.GetModelConfig(); err == nil {
		generator.UpdateModelConfig(savedModelConfig)
		log.Printf("loaded persisted model config provider=%s model=%s", savedModelConfig.Provider, savedModelConfig.Model)
	}
	tts := ai.NewAPITTS(
		cfg.TTSProvider,
		cfg.TTSAPIURL,
		cfg.TTSAPIKey,
		cfg.TTSVoice,
		cfg.TTSModel,
		cfg.TTSAudioFormat,
		cfg.TTSUseUploadPrompt,
		cfg.TTSPromptAudioPath,
		cfg.TTSReturnJSON,
		cfg.TTSTimeoutSeconds,
		cfg.TTSMaxRetries,
	)
	if savedTTSConfig, err := dataStore.GetTTSConfig(); err == nil {
		if savedTTSConfig.AudioFormat == "wav" && cfg.TTSAudioFormat != "" {
			savedTTSConfig.AudioFormat = cfg.TTSAudioFormat
		}
		tts.UpdateTTSConfig(savedTTSConfig)
		log.Printf("loaded persisted tts config provider=%s model=%s voice=%s", savedTTSConfig.Provider, savedTTSConfig.Model, savedTTSConfig.Voice)
	}
	asr := ai.NewAPIASR(cfg.ASRProvider, cfg.ASRAPIURL, cfg.ASRAPIKey, cfg.ASRAppID, cfg.ASRModel)
	if savedASRConfig, err := dataStore.GetASRConfig(); err == nil {
		asr.UpdateASRConfig(savedASRConfig)
		log.Printf("loaded persisted asr config provider=%s model=%s", savedASRConfig.Provider, savedASRConfig.Model)
	}
	configuredMailer := mailservice.NewSMTPMailer(mailservice.Config{
		Host:      cfg.SMTPHost,
		Port:      cfg.SMTPPort,
		Username:  cfg.SMTPUsername,
		Password:  cfg.SMTPPassword,
		From:      cfg.SMTPFrom,
		PublicURL: cfg.PublicAppURL,
		BrandName: "LinguaQuest",
	})
	var authMailer service.AuthMailer
	if configuredMailer.Configured() {
		authMailer = configuredMailer
	} else if cfg.RequireEmailVerification {
		log.Printf("SMTP is not configured; new registrations will require SMTP configuration")
	}
	svc := service.NewWithOptions(dataStore, redisClient, generator, tts, cfg.JWTSecret, service.Options{
		Mailer:                   authMailer,
		PublicAppURL:             cfg.PublicAppURL,
		RequireEmailVerification: cfg.RequireEmailVerification,
		TaskConcurrency:          cfg.GenerationConcurrency,
		TaskTimeout:              time.Duration(cfg.BackgroundTaskTimeoutSeconds) * time.Second,
		Recognizer:               asr,
		Billing: service.BillingOptions{
			OpenSourceEdition:      cfg.IsOpenSourceEdition(),
			MiniProgramEdition:     cfg.IsMiniProgramEdition(),
			Enabled:                cfg.BillingEnabled,
			FreeDailyCredits:       cfg.BillingFreeDailyCredits,
			MiniAdFreeDailyUses:    cfg.MiniAdFreeDailyUses,
			MiniProgramDailyAIUses: cfg.MiniProgramDailyAIUses,
			Timezone:               cfg.BillingTimezone,
			EpayGatewayURL:         cfg.EpayGatewayURL,
			EpayMerchantID:         cfg.EpayMerchantID,
			EpayKey:                cfg.EpayKey,
			EpayNotifyURL:          cfg.EpayNotifyURL,
			DefaultChannel:         cfg.EpayDefaultChannel,
			EpaySignatureMode:      cfg.EpaySignatureMode,
			AdProvider:             cfg.AdProvider,
			AdScriptURL:            cfg.AdScriptURL,
			AdCourseSlot:           cfg.AdCourseSlot,
			AdLibrarySlot:          cfg.AdLibrarySlot,
			AdResultSlot:           cfg.AdResultSlot,
		},
		UsageProtection: service.UsageProtectionOptions{
			Enabled:        cfg.IsMiniProgramEdition(),
			Cooldown:       time.Duration(cfg.MiniProgramCooldownSeconds) * time.Second,
			MaxActiveTasks: cfg.MiniProgramMaxActiveTasks,
		},
		Analytics: analyticsReporter,
	})
	schema, err := graph.NewSchema(svc)
	if err != nil {
		log.Fatalf("failed to build schema: %v", err)
	}
	checker := health.Checker{
		Postgres: pgProber,
		Redis:    redisClient,
		Timeout:  2 * time.Second,
	}
	var paymentNotifier httpserver.PaymentNotifier
	if svc.SubscriptionFeaturesEnabled() {
		paymentNotifier = svc
	}
	securityOptions := httpserver.SecurityOptions{
		GlobalRateLimitPerMinute:    cfg.HTTPRateLimitPerMinute,
		AuthRateLimitPerMinute:      cfg.AuthRateLimitPerMinute,
		AIRequestRateLimitPerMinute: cfg.AIRequestRateLimitPerMinute,
		GraphQLMaxBodyBytes:         int64(cfg.GraphQLMaxBodyBytes),
		MediaProxyMaxBytes:          int64(cfg.MediaProxyMaxBytes),
		TrustProxyHeaders:           cfg.TrustProxyHeaders,
	}
	mux := httpserver.NewMuxWithOptions(schema, cfg.JWTSecret, paymentNotifier, func(ctx context.Context) httpserver.HealthResult {
		result := checker.Check(ctx)
		return httpserver.HealthResult{
			OK:        result.OK,
			Timestamp: result.Timestamp,
			Checks:    result.Checks,
		}
	}, httpserver.MuxOptions{Security: securityOptions, Analytics: analyticsReporter, AnalyticsAdminToken: cfg.AnalyticsAdminToken})
	log.Printf("LinguaQuest API listening on :%s", cfg.Port)
	if err = http.ListenAndServe(":"+cfg.Port, httpserver.WrapWithBaseMiddleware(mux, securityOptions)); err != nil {
		log.Fatal(err)
	}
}
