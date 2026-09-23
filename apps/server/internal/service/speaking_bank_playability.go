package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linguaquest/server/internal/questionbank"
)

// filterPlayableSpeakingBank removes incomplete Part 1 topics and Part 2/3
// groups before random selection. Every item that can be selected must point
// to a non-empty regular file contained by the configured media directory.
func filterPlayableSpeakingBank(bank questionbank.Bank, mediaDir string) (questionbank.Bank, error) {
	playable := make(map[string]bool, len(bank.Items))
	for _, item := range bank.Items {
		playable[item.ID] = speakingAudioFilePlayable(mediaDir, item.AudioURL)
	}

	topics := make(map[string][]questionbank.Item)
	follows := make(map[string][]questionbank.Item)
	cards := make(map[string][]questionbank.Item)
	for _, item := range bank.Items {
		switch item.Part {
		case 1:
			topics[item.Topic] = append(topics[item.Topic], item)
		case 2:
			cards[item.Group] = append(cards[item.Group], item)
		case 3:
			follows[item.Group] = append(follows[item.Group], item)
		}
	}

	filtered := questionbank.Bank{Version: bank.Version, Sources: bank.Sources}
	for _, items := range topics {
		if len(items) < 3 || !allSpeakingItemsPlayable(items, playable) {
			continue
		}
		filtered.Items = append(filtered.Items, items...)
	}
	for group, groupCards := range cards {
		groupFollows := follows[group]
		if len(groupFollows) < 3 || !allSpeakingItemsPlayable(groupFollows, playable) {
			continue
		}
		addedGroup := false
		for _, card := range groupCards {
			if !playable[card.ID] {
				continue
			}
			filtered.Items = append(filtered.Items, card)
			if !addedGroup {
				filtered.Items = append(filtered.Items, groupFollows...)
				addedGroup = true
			}
		}
	}

	if _, err := filtered.SelectExcluding(nil); err != nil {
		return questionbank.Bank{}, fmt.Errorf("题库没有至少两个完整可播放 Part 1 主题和一个完整可播放 Part 2/3 题组: %w", err)
	}
	return filtered, nil
}

func allSpeakingItemsPlayable(items []questionbank.Item, playable map[string]bool) bool {
	for _, item := range items {
		if !playable[item.ID] {
			return false
		}
	}
	return true
}

func speakingAudioFilePlayable(mediaDir, audioURL string) bool {
	path, err := speakingAudioFilePath(mediaDir, audioURL)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func speakingAudioFilePath(mediaDir, audioURL string) (string, error) {
	cleanURL := strings.TrimSpace(audioURL)
	if !strings.HasPrefix(cleanURL, "/media/") || strings.ContainsAny(cleanURL, `\?#`) {
		return "", errors.New("audio URL must be a local /media path")
	}
	relativeURL := strings.TrimPrefix(cleanURL, "/media/")
	if relativeURL == "" || filepath.IsAbs(relativeURL) || filepath.VolumeName(relativeURL) != "" {
		return "", errors.New("audio URL path is empty or absolute")
	}
	for _, part := range strings.Split(relativeURL, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("audio URL path is not safe")
		}
	}

	root, err := filepath.Abs(strings.TrimSpace(mediaDir))
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(relativeURL)))
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	contained, err := filepath.Rel(root, resolved)
	if err != nil || contained == "." || contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
		return "", errors.New("audio file is outside media directory")
	}
	return resolved, nil
}
