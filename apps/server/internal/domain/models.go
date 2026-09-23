package domain

import "time"

type User struct {
	ID            string
	Username      string
	Email         string
	EmailVerified bool
	PasswordHash  string
	Nickname      string
	AvatarURL     string
	Bio           string
	TotalXP       int
	Level         int
	XPIntoLevel   int
	XPToNextLevel int
	LevelProgress int
	RankCode      string
	RankLabel     string
	CreatedAt     time.Time
}

const MaxLevel = 999

type LearningProgress struct {
	Level         int
	XPIntoLevel   int
	XPToNextLevel int
	LevelProgress int
	RankCode      string
	RankLabel     string
}

type XPEvent struct {
	ID        string
	UserID    string
	Activity  string
	SourceID  string
	XPEarned  int
	CreatedAt time.Time
}

type XPAward struct {
	Event     XPEvent
	GrantedXP int
	Duplicate bool
}

type BillingProduct struct {
	Code            string
	Name            string
	Kind            string
	AmountCents     int
	CreditAllowance int
	PeriodDays      int
	AdsFree         bool
	Description     string
}

type BillingStatus struct {
	ProductCode     string
	ProductName     string
	IsLifetime      bool
	AdsFree         bool
	CreditBalance   int
	CreditAllowance int
	CreditResetAt   time.Time
	ExpiresAt       time.Time
}

type AICreditCost struct {
	Action      string
	Label       string
	Credits     int
	Description string
}

type PaymentOrder struct {
	ID              string
	UserID          string
	ProductCode     string
	AmountCents     int
	PaymentChannel  string
	Status          string
	ProviderTradeNo string
	CheckoutURL     string
	CreatedAt       time.Time
	PaidAt          time.Time
}

type AdPlacement struct {
	Placement string
	Provider  string
	ScriptURL string
	SlotID    string
}

type LoginCandidate struct {
	ID       string
	Username string
	Email    string
}

type AuthResult struct {
	AccessToken               string
	RefreshToken              string
	UserID                    string
	EmailVerificationRequired bool
	EmailSent                 bool
	OnboardingRequired        bool
	Message                   string
}

type EmailActionResult struct {
	RequiresSelection bool
	Candidates        []LoginCandidate
	Message           string
}

type ModelConfig struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKey    string
	UpdatedAt time.Time
}

type ModelConfigView struct {
	Provider      string
	Model         string
	BaseURL       string
	HasAPIKey     bool
	APIKeyPreview string
	UpdatedAt     time.Time
}

type ModelConfigUpdate struct {
	Provider    string
	Model       string
	BaseURL     string
	APIKey      string
	ClearAPIKey bool
}

type TTSConfig struct {
	Provider    string
	Model       string
	BaseURL     string
	APIKey      string
	Voice       string
	AudioFormat string
	UpdatedAt   time.Time
}

type TTSConfigView struct {
	Provider      string
	Model         string
	BaseURL       string
	Voice         string
	AudioFormat   string
	HasAPIKey     bool
	APIKeyPreview string
	UpdatedAt     time.Time
}

type TTSConfigUpdate struct {
	Provider    string
	Model       string
	BaseURL     string
	APIKey      string
	Voice       string
	ClearAPIKey bool
}

type OAuthAccount struct {
	ID           string
	Email        string
	Provider     string
	ClientID     string
	RefreshToken string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ASRConfig is kept separately from TTS because providers often use different
// credentials, request formats, and supported languages.
type ASRConfig struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKey    string
	AppID     string
	UpdatedAt time.Time
}

type ASRConfigView struct {
	Provider      string
	Model         string
	BaseURL       string
	HasAPIKey     bool
	APIKeyPreview string
	AppID         string
	UpdatedAt     time.Time
}

type ASRConfigUpdate struct {
	Provider    string
	Model       string
	BaseURL     string
	APIKey      string
	AppID       string
	ClearAPIKey bool
}

type TranscriptResult struct {
	Text              string
	RequestedLanguage string
	DetectedLanguage  string
	DurationSeconds   int
	Provider          string
	Model             string
}

type VoiceProfile struct {
	ID                string
	UserID            string
	Name              string
	Prompt            string
	Language          string
	Provider          string
	Model             string
	PreviewAudioURL   string
	Status            string
	GenerationMessage string
	CreatedAt         time.Time
}

type Dialogue struct {
	Speaker    string
	Gender     string
	Text       string
	ZhSubtitle string
	AudioURL   string
	Timestamp  float64
}

// QuizQuestion pairs a comprehension question with a short reference answer used only server-side for grading.
type QuizQuestion struct {
	Question     string             `json:"question"`
	Options      []string           `json:"options,omitempty"`
	AnswerKey    string             `json:"answerKey"`
	Type         string             `json:"type,omitempty"`
	ParagraphRef string             `json:"paragraphRef,omitempty"`
	Evidence     string             `json:"evidence,omitempty"`
	Headings     []string           `json:"headings,omitempty"`
	Statements   []ReadingStatement `json:"statements,omitempty"`
	SummaryText  string             `json:"summaryText,omitempty"`
	WordBank     []string           `json:"wordBank,omitempty"`
	Answers      []string           `json:"answers,omitempty"`
}

type ReadingStatement struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Answer string `json:"answer"`
}

type Character struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Color string `json:"color"`
}

// QualityCheck is one immutable decision recorded by a production review.
// Unknown or missing checks are intentionally treated as failures by the
// content quality gate.
type QualityCheck struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ProductionApproval authorizes one exact content revision for production.
// Readiness, media generation and legacy booleans are not approval signals.
type ProductionApproval struct {
	Status        string         `json:"status"`
	RubricVersion string         `json:"rubricVersion,omitempty"`
	Reviewer      string         `json:"reviewer,omitempty"`
	ContentHash   string         `json:"contentHash,omitempty"`
	ReviewedAt    time.Time      `json:"reviewedAt,omitempty"`
	Checks        []QualityCheck `json:"checks,omitempty"`
}

type Theater struct {
	ID                 string
	UserID             string
	Language           string
	Topic              string
	Difficulty         float64
	Mode               string
	Status             string
	GenerationProgress int
	GenerationMessage  string
	IsFavorite         bool
	ShareCode          string
	SceneDescription   string
	Characters         []Character
	Dialogues          []Dialogue
	QuizQuestions      []QuizQuestion
	ProductionApproval ProductionApproval
	CreatedAt          time.Time
}

type PracticeResult struct {
	Score        int
	XPEarned     int
	Feedback     string
	CorrectCount int
	TotalCount   int
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
	ID          string
	Name        string
	Domain      string
	Category    string
	Exam        string
	UseCases    []string
	ContentMode string
	Enabled     bool
	Priority    int
}

type ReadingMaterial struct {
	ID                   string
	UserID               string
	Exam                 string
	Language             string
	Level                string
	Topic                string
	Band                 float64
	Stage                string
	Section              string
	SkillFocus           string
	QuestionType         string
	ScenarioFamily       string
	Title                string
	Passage              string
	Vocabulary           []string
	Questions            []QuizQuestion
	SourceIDs            []string
	GenerationNote       string
	AudioURL             string
	AudioURLs            []string
	AudioStatus          string
	Status               string
	GenerationProgress   int
	GenerationMessage    string
	VocabularyItems      []VocabularyItem
	AssociationSentences []string
	GrammarInsights      []GrammarInsight
	ProductionApproval   ProductionApproval
	CreatedAt            time.Time
}

type ReadingGenerationInput struct {
	Exam           string
	Topic          string
	Level          string
	SourceIDs      []string
	Band           float64
	Stage          string
	Section        string
	SkillFocus     string
	QuestionType   string
	ScenarioFamily string
}

type ReadingGenerationRequest struct {
	// RevisionFeedback 只由服务端独立审核产生，不属于用户输入。
	RevisionFeedback string
	Exam             string
	Language         string
	Topic            string
	Level            string
	Band             float64
	Stage            string
	Section          string
	SkillFocus       string
	QuestionType     string
	ScenarioFamily   string
}

type VocabularyItem struct {
	Word     string   `json:"word"`
	POS      string   `json:"pos"`
	Meanings []string `json:"meanings"`
}

type GrammarInsight struct {
	Sentence         string   `json:"sentence"`
	DifficultyPoints []string `json:"difficultyPoints"`
	StudySuggestions []string `json:"studySuggestions"`
}

type ReadingAnalysis struct {
	VocabularyItems      []VocabularyItem `json:"vocabularyItems"`
	AssociationSentences []string         `json:"associationSentences"`
	GrammarInsights      []GrammarInsight `json:"grammarInsights"`
}

type RoleplaySession struct {
	ID                string
	UserID            string
	TheaterID         string
	UserRole          string
	TurnIndex         int
	CurrentScore      int
	Transcript        []Dialogue
	Status            string
	ProcessingMessage string
	FinalFeedback     string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type WritingPrompt struct {
	Title              string
	Instructions       string
	SuggestedWordCount int
}

type WritingEvaluation struct {
	OverallScore      float64
	GrammarScore      float64
	VocabularyScore   float64
	CoherenceScore    float64
	TaskResponseScore float64
	Strengths         []string
	Issues            []string
	Suggestions       []string
	RevisedExcerpt    string
	Summary           string
	Evidence          []string
	BandEstimate      float64
}

type WritingSession struct {
	ID                 string
	UserID             string
	Exam               string
	TimeLimitSeconds   int
	Prompt             WritingPrompt
	Essay              string
	WordCount          int
	Status             string
	ProgressMessage    string
	Evaluation         *WritingEvaluation
	PromptApproval     ProductionApproval
	EvaluationApproval ProductionApproval
	StartedAt          time.Time
	SubmittedAt        time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// MockExam is an immutable exam snapshot for one user attempt. The answer
// keys stay server-side in the same JSON document as the paper.
type MockExam struct {
	ID                        string
	UserID                    string
	Exam                      string
	Status                    string
	CurrentSection            string
	TotalDurationSeconds      int
	Sections                  []MockExamSection
	Result                    *MockExamResult
	StartedAt                 time.Time
	SubmittedAt               time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	EstimatedReadySeconds     int
	GenerationEstimateSamples int
	ProductionApproval        ProductionApproval
	EvaluationApproval        ProductionApproval
}

type MockExamSection struct {
	ListeningDifficulty       *ListeningDifficultyAudit `json:",omitempty"`
	AudioURLs                 []string
	GenerationDurationSeconds int
	PaperVersion              string
	TargetBand                float64
	QualityApproved           bool
	ProductionApproval        ProductionApproval
	Key                       string
	Title                     string
	Skill                     string
	DurationSeconds           int
	Instructions              string
	Passage                   string
	AudioURL                  string
	AudioScript               string
	Questions                 []QuizQuestion
	WritingPrompts            []WritingPrompt
	Answers                   []string
	Responses                 []string
}

// 难度复核由服务端写入，与作答、音频和用户评分分开保存，不直接暴露给学员。
type ListeningDifficultyAudit struct {
	RubricVersion string
	ReviewerModel string
	ContentHash   string
	ReviewedAt    time.Time
	Review        ListeningDifficultyReview
}

type ListeningDifficultyReview struct {
	Approved   bool                      `json:"approved"`
	Confidence string                    `json:"confidence"`
	Feedback   string                    `json:"feedback"`
	Items      []ListeningDifficultyItem `json:"items"`
}

type ListeningDifficultyItem struct {
	Number     int      `json:"number"`
	Level      string   `json:"level"`
	Answer     string   `json:"answer"`
	Evidence   string   `json:"evidence"`
	Reason     string   `json:"reason"`
	Mechanisms []string `json:"mechanisms"`
}

type MockExamResult struct {
	WritingEvaluations    []WritingEvaluation
	TranslationEvaluation *WritingEvaluation
	WritingBand           float64
	WritingTask1Band      float64
	WritingTask2Band      float64
	ReadingCorrect        int
	ReadingTotal          int
	ListeningCorrect      int
	ListeningTotal        int
	ReadingScore          float64
	ListeningScore        float64
	WritingScore          float64
	WritingTask1Score     float64
	WritingTask2Score     float64
	TranslationScore      float64
	EstimatedBand         float64
	TotalScore            float64
	ScoreScale            string
	QualityStatus         string
	Feedback              string
	Strengths             []string
	Weaknesses            []string
	Recommendations       []string
	CompletedAt           time.Time
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

// SpeakingSession 是独立于剧场角色扮演的 IELTS 口语模拟会话。
// 题目和回答按服务端顺序保存，评分只在完整会话结束后生成。
type SpeakingSession struct {
	ID                 string
	UserID             string
	Status             string
	Part               int
	PromptIndex        int
	PendingPromptIndex int
	PreparationEndsAt  time.Time
	AnswerEndsAt       time.Time
	Prompts            []SpeakingPrompt
	Turns              []SpeakingTurn
	Evaluation         *SpeakingEvaluation
	EvaluationApproval ProductionApproval
	ProcessingMessage  string
	LastError          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type SpeakingPrompt struct {
	QuestionID         string
	Source             string
	AudioURL           string
	Part               int
	Question           string
	CueCard            string
	PreparationSec     int
	AnswerSec          int
	ProductionApproval *ProductionApproval `json:",omitempty"`
}

type SpeakingTurn struct {
	Part             int
	PromptIndex      int
	Prompt           string
	Transcript       string
	AudioURL         string
	ExaminerText     string
	ExaminerAudioURL string
	AudioEvidence    bool
	ASRProvider      string
	ASRModel         string
	DurationSec      int
	SubmittedAt      time.Time
}

type SpeakingEvaluation struct {
	FluencyCoherence *float64
	TextCoherence    *float64
	LexicalResource  *float64
	GrammarAccuracy  *float64
	Pronunciation    *float64
	OverallBand      *float64
	IsPartial        bool
	AssessmentMode   string
	Strengths        []string
	Improvements     []string
	Evidence         []string
	Summary          string
}

type SpeakingExaminerReply struct {
	Text     string
	Question string
	CueCard  string
}
