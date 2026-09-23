package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/linguaquest/server/internal/questionbank"
)

// GenerateSpeakingBankAudio 仅供管理员离线运行；串行生成，每条成功后落盘以便续跑。
func (s *Service) GenerateSpeakingBankAudio(ctx context.Context, path string, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, errors.New("每批音频数量须为 1–100")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return 0, errors.New("缺少 ffmpeg，无法验证生成音频")
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 0, errors.New("题库正在更新，或上次进程留下锁文件，请管理员检查")
	}
	lock.Close()
	defer os.Remove(path + ".lock")
	bank, err := questionbank.Load(path)
	if err != nil {
		return 0, err
	}
	completed := 0
	for idx := range bank.Items {
		item := &bank.Items[idx]
		if item.AudioURL != "" {
			continue
		}
		text := item.Question
		if item.Part == 2 {
			text = item.CueCard
		}
		raw, e := s.synthesizeWithTTSLimit(ctx, text, "ENGLISH", "")
		if e != nil {
			return completed, fmt.Errorf("TTS 调用失败，题目 %s；已完成的音频保留，请检查服务配置后重试", item.ID)
		}
		if !strings.HasPrefix(raw, "data:audio/") {
			return completed, errors.New("TTS 未返回内联音频，不能保证持久保存，已停止")
		}
		url, e := s.materializeAudioURL(raw, "question-bank", item.ID)
		if e != nil {
			return completed, errors.New("题库音频保存失败")
		}
		file := filepath.Join(s.mediaDir, filepath.FromSlash(strings.TrimPrefix(url, "/media/")))
		if e = exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-xerror", "-i", file, "-f", "null", "-").Run(); e != nil {
			return completed, fmt.Errorf("音频解码校验失败，题目 %s 未标记为可播放", item.ID)
		}
		item.AudioURL = url
		item.AudioStatus = "GENERATED_TTS"
		data, e := json.MarshalIndent(bank, "", "  ")
		if e != nil {
			return completed, e
		}
		temp, e := os.CreateTemp(filepath.Dir(path), "bank-audio-*.tmp")
		if e != nil {
			return completed, e
		}
		name := temp.Name()
		_, e = temp.Write(data)
		if closeErr := temp.Close(); e == nil {
			e = closeErr
		}
		if e == nil {
			e = os.Rename(name, path)
		}
		if e != nil {
			os.Remove(name)
			return completed, e
		}
		completed++
		if completed >= limit {
			break
		}
	}
	return completed, nil
}
