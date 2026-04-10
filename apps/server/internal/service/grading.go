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
		scene := "City learning scene around topic: " + topic + ", with practical spoken interaction."
		characters := []domain.Character{
			{Name: names[0], Role: "Guide", Color: "#3EA38D"},
		}
		if len(names) > 1 {
			characters = append(characters, domain.Character{Name: names[1], Role: "Learner", Color: "#F4B400"})
		}
		return scene, characters
	default:
		if len(names) == 0 {
			names = []string{"阿明", "小美"}
		}
		scene := "围绕「" + topic + "」展开的港式口语场景练习，强调真实交流与语境理解。"
		characters := []domain.Character{
			{Name: names[0], Role: "引导者", Color: "#FF6F3C"},
		}
		if len(names) > 1 {
			characters = append(characters, domain.Character{Name: names[1], Role: "学习者", Color: "#3AA9D2"})
		}
		return scene, characters
	}
}

func fallbackTheaterContent(language string, topic string) ([]domain.Dialogue, []domain.QuizQuestion) {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "ENGLISH":
		return []domain.Dialogue{
				{Speaker: "Alex", Text: "Welcome to today's mini-theater. We're exploring: " + topic + ".", AudioURL: "https://example.com/audio/1.mp3", Timestamp: 0},
				{Speaker: "Sam", Text: "I'm a bit nervous, but I'm ready to practice.", AudioURL: "https://example.com/audio/2.mp3", Timestamp: 3.2},
				{Speaker: "Alex", Text: "Great—focus on natural phrasing and clear ideas.", AudioURL: "https://example.com/audio/3.mp3", Timestamp: 5.8},
				{Speaker: "Sam", Text: "Got it. I'll try to respond in full sentences.", AudioURL: "https://example.com/audio/4.mp3", Timestamp: 8.1},
			},
			[]domain.QuizQuestion{
				{Question: "What is the main topic of this dialogue?", Options: []string{topic, "Booking a hotel", "Applying for a visa", "Buying a laptop"}, AnswerKey: topic},
				{Question: "How does Sam feel at the beginning?", Options: []string{"excited", "nervous", "angry", "confused"}, AnswerKey: "nervous"},
			}
	default:
		return []domain.Dialogue{
				{Speaker: "阿明", Text: "歡迎來到今天嘅小劇場，我哋會傾：" + topic + "。", AudioURL: "https://example.com/audio/1.mp3", Timestamp: 0},
				{Speaker: "小美", Text: "第一次練習有啲緊張，不過我準備好喇。", AudioURL: "https://example.com/audio/2.mp3", Timestamp: 3.2},
				{Speaker: "阿明", Text: "重點係講得自然、意思清楚就得。", AudioURL: "https://example.com/audio/3.mp3", Timestamp: 5.8},
				{Speaker: "小美", Text: "明白，我試下用完整句子回答。", AudioURL: "https://example.com/audio/4.mp3", Timestamp: 8.1},
			},
			[]domain.QuizQuestion{
				{Question: "这段对话主要讨论什么主题？", Options: []string{topic, "申请工作签证", "买笔记本电脑", "租房搬家"}, AnswerKey: topic},
				{Question: "小美一开始是什么感受？", Options: []string{"兴奋", "紧张", "愤怒", "冷漠"}, AnswerKey: "紧张"},
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
