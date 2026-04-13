package domain

import "time"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Nickname     string
	AvatarURL    string
	Bio          string
	TotalXP      int
	CreatedAt    time.Time
}

type Dialogue struct {
	Speaker   string
	Text      string
	ZhSubtitle string
	AudioURL  string
	Timestamp float64
}

// QuizQuestion pairs a comprehension question with a short reference answer used only server-side for grading.
type QuizQuestion struct {
	Question  string `json:"question"`
	Options   []string `json:"options,omitempty"`
	AnswerKey string `json:"answerKey"`
}

type Character struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Color string `json:"color"`
}

type Theater struct {
	ID               string
	UserID           string
	Language         string
	Topic            string
	Difficulty       float64
	Mode             string
	Status           string
	IsFavorite       bool
	ShareCode        string
	SceneDescription string
	Characters       []Character
	Dialogues        []Dialogue
	QuizQuestions    []QuizQuestion
	CreatedAt        time.Time
}

type PracticeResult struct {
	Score         int
	XPEarned      int
	Feedback      string
	CorrectCount  int
	TotalCount    int
}

type Course struct {
	ID          string
	Language    string
	Category    string
	Title       string
	Description string
	MinLevel    float64
	MaxLevel    float64
	IsActive    bool
}

type ContentSource struct {
	ID           string
	Name         string
	Domain       string
	Category     string
	Exam         string
	UseCases     []string
	ContentMode  string
	Enabled      bool
	Priority     int
}

type ReadingMaterial struct {
	ID             string
	UserID         string
	Exam           string
	Language       string
	Level          string
	Topic          string
	Title          string
	Passage        string
	Vocabulary     []string
	Questions      []QuizQuestion
	SourceIDs      []string
	GenerationNote string
	AudioURL       string
	AudioURLs      []string
	AudioStatus    string
	CreatedAt      time.Time
}

type RoleplaySession struct {
	ID            string
	UserID        string
	TheaterID     string
	UserRole      string
	TurnIndex     int
	CurrentScore  int
	Transcript    []Dialogue
	Status        string
	FinalFeedback string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RoleplayTurnEval struct {
	AssistantReply string
	AssistantZhSub string
	Relevance      int
	Accuracy       int
	Naturalness    int
	Total          int
	Feedback       string
}
