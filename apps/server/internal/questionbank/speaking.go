// Package questionbank 读取独立部署的已授权题库，不把原始教材编译进应用。
package questionbank

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/linguaquest/server/internal/domain"
)

type Item struct {
	ID             string          `json:"id"`
	Part           int             `json:"part"`
	Topic          string          `json:"topic"`
	Group          string          `json:"group"`
	Question       string          `json:"question"`
	CueCard        string          `json:"cueCard"`
	PreparationSec int             `json:"preparationSec"`
	AnswerSec      int             `json:"answerSec"`
	Source         string          `json:"source"`
	SourcePage     int             `json:"sourcePage"`
	AudioURL       string          `json:"audioUrl"`
	AudioStatus    string          `json:"audioStatus"`
	AudioSource    string          `json:"audioSource,omitempty"`
	Sources        json.RawMessage `json:"sources,omitempty"`
}
type Bank struct {
	Version int             `json:"version"`
	Sources json.RawMessage `json:"sources"`
	Items   []Item          `json:"items"`
}

func Load(path string) (Bank, error) {
	var b Bank
	data, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	if len(data) > 20*1024*1024 {
		return b, errors.New("题库文件过大")
	}
	if err = json.Unmarshal(data, &b); err != nil {
		return b, errors.New("题库 JSON 格式无效")
	}
	if b.Version != 1 || len(b.Items) == 0 {
		return b, errors.New("题库版本无效或内容为空")
	}
	seen := map[string]bool{}
	for _, i := range b.Items {
		if i.ID == "" || seen[i.ID] || i.Source == "" || i.SourcePage < 1 || i.Part < 1 || i.Part > 3 {
			return b, errors.New("题库标识或来源不完整")
		}
		seen[i.ID] = true
		if i.Part == 2 {
			if !strings.Contains(i.CueCard, "You should say:") || i.Group == "" {
				return b, errors.New("题卡不完整")
			}
		} else if !strings.HasSuffix(i.Question, "?") {
			return b, errors.New("口语问题不完整")
		}
		if i.AudioURL != "" && (!strings.HasPrefix(i.AudioURL, "/media/") || strings.Contains(i.AudioURL, "..") || strings.ContainsAny(i.AudioURL, "\\?#")) {
			return b, errors.New("题库音频必须是本服务媒体地址")
		}
	}
	return b, nil
}

func choose(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

// SpeakingSelectionFingerprint identifies a prompt set regardless of the order
// in which its Part 1 topics were selected.
func SpeakingSelectionFingerprint(prompts []domain.SpeakingPrompt) string {
	ids := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		id := strings.TrimSpace(prompt.QuestionID)
		if id == "" {
			return ""
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, "\x1f")
}

func selectedPrompts(items []Item) []domain.SpeakingPrompt {
	prompts := make([]domain.SpeakingPrompt, 0, len(items))
	for _, i := range items {
		prompts = append(prompts, domain.SpeakingPrompt{Part: i.Part, Question: i.Question, CueCard: i.CueCard, PreparationSec: i.PreparationSec, AnswerSec: i.AnswerSec, QuestionID: i.ID, Source: fmt.Sprintf("%s · 第 %d 页", i.Source, i.SourcePage), AudioURL: i.AudioURL})
	}
	return prompts
}

// Select 保留题卡与深入追问的配套关系，Part 1 按两个主题抽取。
func (b Bank) Select() ([]domain.SpeakingPrompt, error) {
	return b.SelectExcluding(nil)
}

// SelectExcluding selects a complete prompt set that has not already been
// attempted. A deterministic fallback prevents random collisions from making
// an available set look exhausted.
func (b Bank) SelectExcluding(excluded map[string]struct{}) ([]domain.SpeakingPrompt, error) {
	return b.selectExcluding(excluded, nil)
}

// SelectExcludingItems selects a complete prompt set while excluding both
// previously attempted selections and prompt IDs rejected by production review.
func (b Bank) SelectExcludingItems(excluded, excludedItemIDs map[string]struct{}) ([]domain.SpeakingPrompt, error) {
	return b.selectExcluding(excluded, excludedItemIDs)
}

func (b Bank) selectExcluding(excluded, excludedItemIDs map[string]struct{}) ([]domain.SpeakingPrompt, error) {
	topics := map[string][]Item{}
	follows := map[string][]Item{}
	var cards []Item
	for _, i := range b.Items {
		switch i.Part {
		case 1:
			topics[i.Topic] = append(topics[i.Topic], i)
		case 2:
			cards = append(cards, i)
		case 3:
			follows[i.Group] = append(follows[i.Group], i)
		}
	}
	var names []string
	for n, items := range topics {
		if len(items) >= 3 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var eligible []Item
	for _, c := range cards {
		if len(follows[c.Group]) >= 3 {
			eligible = append(eligible, c)
		}
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	if len(names) < 2 || len(eligible) == 0 {
		return nil, errors.New("题库缺少完整 Part 1 或 Part 2/3 配套题")
	}
	build := func(first, second string, card Item) []domain.SpeakingPrompt {
		selected := make([]Item, 0, 7+len(follows[card.Group]))
		selected = append(selected, topics[first][:3]...)
		selected = append(selected, topics[second][:3]...)
		selected = append(selected, card)
		selected = append(selected, follows[card.Group]...)
		return selectedPrompts(selected)
	}
	available := func(prompts []domain.SpeakingPrompt) bool {
		if _, found := excluded[SpeakingSelectionFingerprint(prompts)]; found {
			return false
		}
		for _, prompt := range prompts {
			if _, found := excludedItemIDs[strings.TrimSpace(prompt.QuestionID)]; found {
				return false
			}
		}
		return true
	}
	for attempt := 0; attempt < 32; attempt++ {
		firstIndex := choose(len(names))
		secondIndex := choose(len(names) - 1)
		if secondIndex >= firstIndex {
			secondIndex++
		}
		prompts := build(names[firstIndex], names[secondIndex], eligible[choose(len(eligible))])
		if available(prompts) {
			return prompts, nil
		}
	}
	for first := 0; first < len(names)-1; first++ {
		for second := first + 1; second < len(names); second++ {
			for _, card := range eligible {
				prompts := build(names[first], names[second], card)
				if available(prompts) {
					return prompts, nil
				}
			}
		}
	}
	return nil, errors.New("题库中没有尚未尝试的完整口语题组")
}
