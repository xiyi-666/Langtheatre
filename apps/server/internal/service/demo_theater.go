package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const demoCantoneseAudioOwner = "efc3cd91-e319-4939-97d7-5f9a93efa454"

const (
	demoAccountPassword = "LqDemo2026!"
	demoAccountEmail    = "demo@linguaquest.local"
)

var demoCantoneseAudio = []string{
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/68528b9f35440c9bd95637850f839e0acc02b76d38d10e0ff665d67a2d3916c7.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/cf7f1d4683105bd0861acaad2cc7f9664fcf10c8be713a13e6e883946aba17c9.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/cf4091d8d8d8cb4e6f4d38f67049f60a37f4bc2173d546e615eed0fe31a953a4b.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/b88fef3303aeee253aecbc0641503962d6bd18cd74b9f84744e7ab9a43bb7377.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/52d8c6fe3d53fc131f40769221695ce8751bea42366b07c24964897a84400cfa.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/4ae908c2cb2e1e50f9424e604c2da19ef78be04637d48cf4d81f3fdf9e18ac3b.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/35da5fe6f1ed162bc1f5d484ea2dbb77852216a9b73497e33d87d4223202cb26.mp3",
	"/media/tts/theater/" + demoCantoneseAudioOwner + "/f4b689c79480ed7e4593d97b37ebe4a33193ccaf139a894104f0704a2d29196d.mp3",
}

// EnsureDemoFixtures 将演示账号的基础剧场写入当前数据存储，重复启动不会重复创建。
func (s *Service) EnsureDemoFixtures() error {
	user, err := s.store.GetUserByUsername(demoAccountUsername)
	if err != nil {
		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(demoAccountPassword), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		user, err = s.store.CreateUser(demoAccountUsername, demoAccountEmail, string(passwordHash), true)
		if err != nil {
			return err
		}
	}
	existing, err := s.store.ListTheatersByUser(user.ID, "", "", nil)
	if err != nil {
		return err
	}
	for _, fixture := range demoTheaterFixtures(user.ID) {
		if demoTheaterExists(existing, fixture.Language, fixture.Topic) {
			continue
		}
		if _, err := s.store.SaveTheater(fixture); err != nil {
			return err
		}
	}
	difficulty, err := s.ensureDemoDifficulty(user.ID)
	if err != nil {
		return err
	}
	if err := s.ensureDemoLearningFixtures(user.ID, difficulty); err != nil {
		return err
	}
	if err := s.ensureDemoVoiceProfiles(user.ID); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensureDemoVoiceProfiles(userID string) error {
	profiles, err := s.store.ListVoiceProfiles(userID)
	if err != nil {
		return err
	}
	existingModels := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		if strings.EqualFold(strings.TrimSpace(profile.Provider), "DEMO") {
			existingModels[strings.TrimSpace(profile.Model)] = true
		}
	}

	now := time.Now().UTC()
	fixtures := []domain.VoiceProfile{
		{ID: uuid.NewString(), UserID: userID, Name: "演示 · 粤语女声", Prompt: "香港年轻女性，亲切自然，粤语语流清晰，适合日常对话。", Language: "CANTONESE", Provider: "DEMO", Model: "preset-female-cantonese", PreviewAudioURL: demoCantoneseAudio[1], Status: "READY", GenerationMessage: "演示音色已准备，不调用 TTS，可直接用于体验。", CreatedAt: now},
		{ID: uuid.NewString(), UserID: userID, Name: "演示 · 粤语男声", Prompt: "香港年轻男性，沉稳温暖，粤语发音自然，适合角色对话。", Language: "CANTONESE", Provider: "DEMO", Model: "preset-male-cantonese", PreviewAudioURL: demoCantoneseAudio[0], Status: "READY", GenerationMessage: "演示音色已准备，不调用 TTS，可直接用于体验。", CreatedAt: now.Add(time.Millisecond)},
		{ID: uuid.NewString(), UserID: userID, Name: "演示 · 英语女声", Prompt: "Warm, clear female English voice with natural pacing for everyday learning conversations.", Language: "ENGLISH", Provider: "DEMO", Model: "preset-female-english", Status: "READY", GenerationMessage: "演示英语音色已准备，不调用 TTS，可直接用于体验。", CreatedAt: now.Add(2 * time.Millisecond)},
		{ID: uuid.NewString(), UserID: userID, Name: "演示 · 英语男声", Prompt: "A calm, friendly male English voice with clear diction for practical roleplay.", Language: "ENGLISH", Provider: "DEMO", Model: "preset-male-english", Status: "READY", GenerationMessage: "演示英语音色已准备，不调用 TTS，可直接用于体验。", CreatedAt: now.Add(3 * time.Millisecond)},
	}
	for _, profile := range fixtures {
		if existingModels[profile.Model] {
			continue
		}
		if _, err := s.store.SaveVoiceProfile(profile); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) createDemoVoiceProfile(userID, name, prompt, language string) (domain.VoiceProfile, error) {
	name = strings.TrimSpace(name)
	prompt = strings.TrimSpace(prompt)
	language = strings.ToUpper(strings.TrimSpace(language))
	if len([]rune(name)) < 2 || len([]rune(name)) > 40 {
		return domain.VoiceProfile{}, errors.New("音色名称须为 2–40 个字符。")
	}
	if len([]rune(prompt)) < 8 || len([]rune(prompt)) > 500 {
		return domain.VoiceProfile{}, errors.New("音色描述须为 8–500 个字符。")
	}
	if language != "CANTONESE" && language != "ENGLISH" {
		return domain.VoiceProfile{}, errors.New("音色语言只能选择粤语或英语。")
	}
	previewURL := ""
	if language == "CANTONESE" {
		previewURL = demoCantoneseAudio[0]
	}
	profile := domain.VoiceProfile{
		ID:                uuid.NewString(),
		UserID:            userID,
		Name:              name,
		Prompt:            prompt,
		Language:          language,
		Provider:          "DEMO",
		Model:             "preset-voice-design",
		PreviewAudioURL:   previewURL,
		Status:            "READY",
		GenerationMessage: "演示音色已生成，不调用 TTS，也不会消耗 AI 点数。",
		CreatedAt:         time.Now().UTC(),
	}
	return s.store.SaveVoiceProfile(profile)
}

func (s *Service) startDemoRoleplay(userID, theaterID, userRole string) (domain.RoleplaySession, error) {
	theater, err := s.store.GetTheater(theaterID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if err = ensureTheaterReady(theater); err != nil {
		return domain.RoleplaySession{}, err
	}
	if strings.TrimSpace(userRole) == "" {
		return domain.RoleplaySession{}, errors.New("请先填写你的角色。")
	}
	opening := domain.Dialogue{Speaker: "AI-Role", Text: "你好，我们开始角色扮演。请先用一句话介绍你的立场。", ZhSubtitle: "你好，我们开始角色扮演。请先用一句话介绍你的立场。"}
	if len(theater.Dialogues) > 0 {
		opening = theater.Dialogues[0]
		opening.Speaker = "AI-Role"
	}
	now := time.Now().UTC()
	session := domain.RoleplaySession{
		ID:                uuid.NewString(),
		UserID:            userID,
		TheaterID:         theaterID,
		UserRole:          strings.TrimSpace(userRole),
		Status:            "active",
		ProcessingMessage: "演示模式：使用预置对话，不调用模型、ASR 或 TTS。",
		Transcript:        []domain.Dialogue{opening},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	saved, err := s.store.CreateRoleplaySession(session)
	if err == nil {
		s.trackFeature("ROLEPLAY_SESSION_STARTED_DEMO")
	}
	return saved, err
}

func (s *Service) submitDemoRoleplayReply(userID, sessionID, text string) (domain.RoleplaySession, error) {
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(session.Status), "active") {
		return domain.RoleplaySession{}, errors.New("角色扮演会话已结束，请重新开始。")
	}
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return domain.RoleplaySession{}, errors.New("回复内容不能为空。")
	}
	theater, err := s.store.GetTheater(session.TheaterID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	session.TurnIndex++
	session.Transcript = append(session.Transcript, domain.Dialogue{Speaker: "USER", Text: cleanText, Timestamp: float64(session.TurnIndex)})
	eval := fallbackRoleplayTurn(theater.Language, cleanText)
	responseIndex := session.TurnIndex*2 - 1
	if responseIndex >= 0 && responseIndex < len(theater.Dialogues) {
		fixture := theater.Dialogues[responseIndex]
		eval.AssistantReply = fixture.Text
		eval.AssistantZhSub = fixture.ZhSubtitle
	}
	session.CurrentScore = ((session.CurrentScore * (session.TurnIndex - 1)) + eval.Total) / session.TurnIndex
	audioURL := ""
	if responseIndex >= 0 && responseIndex < len(theater.Dialogues) {
		audioURL = theater.Dialogues[responseIndex].AudioURL
	}
	session.Transcript = append(session.Transcript, domain.Dialogue{Speaker: "AI-Role", Text: buildTurnFeedbackText(theater.Language, eval), ZhSubtitle: eval.AssistantZhSub, AudioURL: audioURL, Timestamp: float64(session.TurnIndex) + 0.3})
	session.ProcessingMessage = "演示模式：已使用预置回复，不调用模型或 TTS。"
	session.UpdatedAt = time.Now().UTC()
	saved, err := s.store.UpdateRoleplaySession(session)
	if err == nil {
		_, _ = s.awardLearningXP(userID, "SPEAKING_TURN", fmt.Sprintf("%s:%d", sessionID, session.TurnIndex), eval.Total)
		s.trackFeature("ROLEPLAY_TEXT_TURN_COMPLETED_DEMO")
	}
	return saved, err
}

func (s *Service) submitDemoRoleplayAudio(userID, sessionID, language string) (domain.RoleplaySession, error) {
	text := "演示语音识别：我想练习这个场景的表达。"
	if strings.EqualFold(strings.TrimSpace(language), "ENGLISH") {
		text = "Demo transcription: I would like to practise this situation."
	}
	updated, err := s.submitDemoRoleplayReply(userID, sessionID, text)
	if err == nil {
		updated.ProcessingMessage = "演示模式：已使用预置识别文本与回复，不调用 ASR 或 TTS。"
		updated, err = s.store.UpdateRoleplaySession(updated)
	}
	return updated, err
}

func demoTheaterExists(items []domain.Theater, language string, topic string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Language), language) && strings.EqualFold(strings.TrimSpace(item.Topic), topic) {
			return true
		}
	}
	return false
}

func (s *Service) generateDemoTheater(userID string, language string, topic string, difficulty float64, mode string) (domain.Theater, error) {
	fixtures := demoTheaterFixtures(userID)
	assigned, err := s.ensureDemoDifficulty(userID)
	if err != nil {
		return domain.Theater{}, err
	}
	assignedBand := parseDemoDifficulty(assigned)
	if isValidDemoDifficulty(difficulty) {
		assignedBand = difficulty
	}
	selected := fixtures[0]
	bestDistance := math.MaxFloat64
	for _, fixture := range fixtures {
		if !strings.EqualFold(strings.TrimSpace(fixture.Language), strings.TrimSpace(language)) {
			continue
		}
		distance := math.Abs(fixture.Difficulty - assignedBand)
		if distance < bestDistance {
			selected = fixture
			bestDistance = distance
		}
	}
	selected.ID = uuid.NewString()
	selected.Difficulty = assignedBand
	if strings.TrimSpace(topic) != "" {
		selected.Topic = strings.TrimSpace(topic)
	}
	if strings.TrimSpace(mode) != "" {
		selected.Mode = mode
	}
	selected.CreatedAt = time.Now().UTC()
	if saved, err := s.store.SaveTheater(selected); err != nil {
		return domain.Theater{}, err
	} else {
		return saved, nil
	}
}

func demoTheaterBaseFixtures(userID string) []domain.Theater {
	now := time.Now().UTC()
	cantoneseText := []struct {
		speaker string
		gender  string
		text    string
		sub     string
	}{
		{"阿明", "MALE", "喂，你今日收工之后得唔得閒飲杯奶茶？", "你今天下班后有空喝杯奶茶吗？"},
		{"阿May", "FEMALE", "得呀，六点半喺地铁站出口等你。", "可以呀，六点半在地铁站出口等你。"},
		{"阿明", "MALE", "好啊，不过我今日可能会迟少少。", "好啊，不过我今天可能会晚一点。"},
		{"阿May", "FEMALE", "唔紧要，我顺便去隔离间书店行吓。", "没关系，我正好去旁边的书店逛逛。"},
		{"阿明", "MALE", "咁我哋一阵见，记得帮我少甜呀。", "那我们一会儿见，记得帮我点少糖的。"},
		{"阿May", "FEMALE", "知道喇，凍奶茶少甜，唔会搞错。", "知道啦，少糖冻奶茶，不会弄错。"},
		{"阿明", "MALE", "如果我早到，就帮你买份菠萝包。", "如果我早到，就帮你买一份菠萝包。"},
		{"阿May", "FEMALE", "好呀，见面再慢慢倾，唔使急住走。", "好呀，见面后再慢慢聊，不用急着走。"},
	}
	englishText := []struct {
		speaker string
		gender  string
		text    string
		sub     string
	}{
		{"Maya", "FEMALE", "Are you still coming to the study group after work?", "你下班后还来学习小组吗？"},
		{"Daniel", "MALE", "I am, but I may arrive ten minutes late because of the rain.", "我会来，不过下雨可能会迟到十分钟。"},
		{"Maya", "FEMALE", "That is fine. I will save you a seat near the window.", "没关系，我会在窗边帮你留一个座位。"},
		{"Daniel", "MALE", "Thanks. I have prepared a few questions about the article.", "谢谢，我准备了几个关于文章的问题。"},
		{"Maya", "FEMALE", "We can compare our answers before the discussion starts.", "讨论开始前我们可以先对一下答案。"},
		{"Daniel", "MALE", "Good idea. I also want to practise explaining my opinion clearly.", "好主意，我还想练习清楚地表达自己的观点。"},
		{"Maya", "FEMALE", "Then let us begin with the main reason, and add an example.", "那我们先说主要原因，再补充一个例子。"},
		{"Daniel", "MALE", "Perfect. A simple structure will make the conversation easier to follow.", "很好，简单的结构会让对话更容易理解。"},
	}
	makeDialogues := func(items []struct {
		speaker string
		gender  string
		text    string
		sub     string
	}, audio []string) []domain.Dialogue {
		result := make([]domain.Dialogue, 0, len(items))
		for index, item := range items {
			dialogue := domain.Dialogue{Speaker: item.speaker, Gender: item.gender, Text: item.text, ZhSubtitle: item.sub, Timestamp: float64(index) * 2}
			if index < len(audio) {
				dialogue.AudioURL = audio[index]
			}
			result = append(result, dialogue)
		}
		return result
	}

	return []domain.Theater{
		{
			ID: uuid.NewString(), UserID: userID, Language: "CANTONESE", Topic: "茶餐厅点餐与约会", Difficulty: 5.5, Mode: "LISTENING", Status: "READY", GenerationProgress: 100, GenerationMessage: "演示内容已准备，无需调用 AI", SceneDescription: "茶餐厅下班后的自然粤语双人对话。", Characters: []domain.Character{{Name: "阿明", Role: "男声顾客", Color: "#D98262"}, {Name: "阿May", Role: "女声朋友", Color: "#3D8B7D"}}, Dialogues: makeDialogues(cantoneseText, demoCantoneseAudio), QuizQuestions: []domain.QuizQuestion{{Question: "阿May 会在几点等阿明？", Options: []string{"五点半", "六点半", "七点半", "八点整"}, AnswerKey: "六点半"}, {Question: "阿明想要什么口味的奶茶？", Options: []string{"全糖", "热饮无糖", "冻奶茶少甜", "柠檬茶少冰"}, AnswerKey: "冻奶茶少甜"}, {Question: "阿May 等待时打算做什么？", Options: []string{"去书店逛逛", "先回家", "去买电影票", "去街市买菜"}, AnswerKey: "去书店逛逛"}}, CreatedAt: now,
		},
		{
			ID: uuid.NewString(), UserID: userID, Language: "ENGLISH", Topic: "Study group discussion", Difficulty: 5.5, Mode: "LISTENING", Status: "READY", GenerationProgress: 100, GenerationMessage: "演示内容已准备，无需调用 AI", SceneDescription: "An English study-group conversation about organising a discussion.", Characters: []domain.Character{{Name: "Maya", Role: "Female student", Color: "#D98262"}, {Name: "Daniel", Role: "Male student", Color: "#3D8B7D"}}, Dialogues: makeDialogues(englishText, nil), QuizQuestions: []domain.QuizQuestion{{Question: "Why might Daniel arrive late?", Options: []string{"He has a class", "It is raining", "He missed the discussion", "He is buying a book"}, AnswerKey: "It is raining"}, {Question: "What will Maya save for Daniel?", Options: []string{"A book", "A question", "A seat", "An example"}, AnswerKey: "A seat"}, {Question: "What structure do they plan to practise?", Options: []string{"Reason then example", "Question then apology", "Summary then translation", "Greeting then silence"}, AnswerKey: "Reason then example"}}, CreatedAt: now,
		},
	}
}

// demoTheaterFixtures 提供三档难度的双语剧场。低档保留真实粤语音频，
// 其余台词由前端使用 zh-HK 语音兜底播放，保证演示内容始终可试听。
func demoTheaterFixtures(userID string) []domain.Theater {
	base := demoTheaterBaseFixtures(userID)
	result := make([]domain.Theater, 0, 6)
	for _, item := range base {
		result = append(result, item)
		for _, band := range []float64{6.5, 7.5} {
			copy := item
			copy.ID = uuid.NewString()
			copy.Dialogues = append([]domain.Dialogue(nil), item.Dialogues...)
			copy.Characters = append([]domain.Character(nil), item.Characters...)
			copy.QuizQuestions = append([]domain.QuizQuestion(nil), item.QuizQuestions...)
			copy.Difficulty = band
			copy.Topic = fmt.Sprintf("%s · Band %.1f", item.Topic, band)
			copy.SceneDescription = fmt.Sprintf("%s · %.1f 难度档，包含更长的回应、原因解释和自然衔接。", item.SceneDescription, band)
			for index := range copy.Dialogues {
				copy.Dialogues[index].AudioURL = ""
			}
			if band >= 7 {
				copy.Dialogues = append(copy.Dialogues,
					domain.Dialogue{Speaker: copy.Characters[0].Name, Gender: "MALE", Text: "如果你想改时间，最好早啲同我讲，咁样安排会顺利好多。", ZhSubtitle: "如果你想改时间，最好早点告诉我，这样安排会顺利很多。", Timestamp: 16},
					domain.Dialogue{Speaker: copy.Characters[1].Name, Gender: "FEMALE", Text: "明白，我会先确认交通情况，再同你讲最后决定。", ZhSubtitle: "明白，我会先确认交通情况，再告诉你最后决定。", Timestamp: 18},
				)
			}
			result = append(result, copy)
		}
	}
	return result
}
