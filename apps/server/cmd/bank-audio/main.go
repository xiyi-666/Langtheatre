// bank-audio 使用后端 .env 及持久化 TTS 配置补充基础题库音频。
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/config"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
)

func main() {
	path := flag.String("bank", "data/question-bank/speaking.json", "题库 JSON 路径")
	limit := flag.Int("limit", 3, "本次最大生成数量（1–100）")
	flag.Parse()
	cfg := config.Load()
	tts := ai.NewAPITTS(cfg.TTSProvider, cfg.TTSAPIURL, cfg.TTSAPIKey, cfg.TTSVoice, cfg.TTSModel, cfg.TTSAudioFormat, cfg.TTSUseUploadPrompt, cfg.TTSPromptAudioPath, cfg.TTSReturnJSON, cfg.TTSTimeoutSeconds, cfg.TTSMaxRetries)
	var saved domain.TTSConfig
	var err error
	if cfg.DatabaseURL != "" {
		db, e := store.NewPostgresStore(cfg.DatabaseURL)
		if e != nil {
			log.Fatal("无法连接 PostgreSQL，停止生成")
		}
		defer db.Close()
		saved, err = db.GetTTSConfig()
	} else if cfg.SQLitePath != "" {
		db, e := store.NewSQLiteStore(cfg.SQLitePath)
		if e != nil {
			log.Fatal("无法打开 SQLite，停止生成")
		}
		defer db.Close()
		saved, err = db.GetTTSConfig()
	}
	if err == nil && saved.Provider != "" {
		tts.UpdateTTSConfig(saved)
	}
	if tts.GetTTSConfig().APIKey == "" {
		log.Fatal("TTS 密钥尚未配置")
	}
	svc := service.NewWithOptions(nil, nil, nil, tts, "", service.Options{MediaDir: cfg.MediaDir, TTSMaxConcurrency: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	n, e := svc.GenerateSpeakingBankAudio(ctx, *path, *limit)
	log.Printf("本批完成 %d 条音频", n)
	if e != nil {
		log.Fatal(e)
	}
}
