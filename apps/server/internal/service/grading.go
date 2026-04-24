package service

import (
	"strings"

	"github.com/linguaquest/server/internal/domain"
)

func fallbackSceneAndCharacters(language string, topic string, dialogues []domain.Dialogue) (string, []domain.Character) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		topic = "日常沟通"
	}
	seen := map[string]bool{}
	names := make([]string, 0, 2)
	for _, d := range dialogues {
		name := strings.TrimSpace(d.Speaker)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
		if len(names) >= 2 {
			break
		}
	}
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "ENGLISH":
		if len(names) == 0 {
			names = []string{"Alex", "Sam"}
		}
		scene := "A real-life city scenario connected to " + topic + ", with timing pressure and a practical decision to make."
		characters := []domain.Character{
			{Name: names[0], Role: "Coordinator", Color: "#3EA38D"},
		}
		if len(names) > 1 {
			characters = append(characters, domain.Character{Name: names[1], Role: "Participant", Color: "#F4B400"})
		}
		return scene, characters
	default:
		if len(names) == 0 {
			names = []string{"阿明", "小美"}
		}
		scene := "一个围绕「" + topic + "」展开的香港日常具体情境，需要即时沟通并解决实际问题。"
		characters := []domain.Character{
			{Name: names[0], Role: "协调者", Color: "#FF6F3C"},
		}
		if len(names) > 1 {
			characters = append(characters, domain.Character{Name: names[1], Role: "参与者", Color: "#3AA9D2"})
		}
		return scene, characters
	}
}

func fallbackTheaterContent(language string, topic string) ([]domain.Dialogue, []domain.QuizQuestion) {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "ENGLISH":
		return []domain.Dialogue{
				{Speaker: "Alex", Text: "The station app says the next train is delayed again, so we need another way to get there on time.", AudioURL: "https://example.com/audio/1.mp3", Timestamp: 0},
				{Speaker: "Sam", Text: "If we leave in the next two minutes, bus 18 plus a short walk might still work.", AudioURL: "https://example.com/audio/2.mp3", Timestamp: 3.2},
				{Speaker: "Alex", Text: "Then message the client now and tell them our revised arrival time before we transfer.", AudioURL: "https://example.com/audio/3.mp3", Timestamp: 5.8},
				{Speaker: "Sam", Text: "Done. They said a brief delay is fine as long as we keep them updated.", AudioURL: "https://example.com/audio/4.mp3", Timestamp: 8.1},
			},
			[]domain.QuizQuestion{
				{Question: "What problem do the speakers face?", Options: []string{"A delayed train", "A hotel cancellation", "A broken laptop", "A missing passport"}, AnswerKey: "A delayed train"},
				{Question: "What do they do before changing transport?", Options: []string{"Wait silently", "Message the client", "Book a hotel", "Cancel the meeting"}, AnswerKey: "Message the client"},
			}
	default:
		return []domain.Dialogue{
				{Speaker: "阿明", Text: "港鐵又延誤，照而家咁睇，我哋未必赶得切原本個约定时间。", AudioURL: "https://example.com/audio/1.mp3", Timestamp: 0},
				{Speaker: "小美", Text: "如果即刻转巴士再行过去，应该仲有机会早几分钟到。", AudioURL: "https://example.com/audio/2.mp3", Timestamp: 3.2},
				{Speaker: "阿明", Text: "咁你而家先同对方讲声，顺便报埋新嘅预计到达时间。", AudioURL: "https://example.com/audio/3.mp3", Timestamp: 5.8},
				{Speaker: "小美", Text: "我已经发咗讯息，对方话只要保持更新就冇问题。", AudioURL: "https://example.com/audio/4.mp3", Timestamp: 8.1},
			},
			[]domain.QuizQuestion{
				{Question: "两个人遇到的直接问题是什么？", Options: []string{"地铁延误", "酒店订错", "电脑死机", "签证过期"}, AnswerKey: "地铁延误"},
				{Question: "他们先做了哪一步？", Options: []string{"直接取消约会", "先通知对方新的到达时间", "先回家再说", "先改去别的地方"}, AnswerKey: "先通知对方新的到达时间"},
			}
	}
}

func fallbackQuizOnly(language string, topic string) []domain.QuizQuestion {
	_, q := fallbackTheaterContent(language, topic)
	return q
}

func answerMatches(userAnswer, expected string, language string) bool {
	u := strings.TrimSpace(userAnswer)
	e := strings.TrimSpace(expected)
	if u == "" || e == "" {
		return false
	}
	u = strings.Join(strings.Fields(u), " ")
	e = strings.Join(strings.Fields(e), " ")
	if strings.EqualFold(u, e) {
		return true
	}
	if strings.ToUpper(strings.TrimSpace(language)) == "ENGLISH" {
		lu, le := strings.ToLower(u), strings.ToLower(e)
		if lu == le {
			return true
		}
		if len(le) >= 2 && (strings.Contains(lu, le) || strings.Contains(le, lu)) {
			return true
		}
		return false
	}
	if u == e {
		return true
	}
	if len([]rune(e)) >= 1 && (strings.Contains(u, e) || strings.Contains(e, u)) {
		return true
	}
	return false
}
