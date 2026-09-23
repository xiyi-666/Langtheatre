export type TheaterStatus = "GENERATING" | "READY" | "FAILED";

export interface User {
  id: string;
  username: string;
  email: string;
  emailVerified: boolean;
  nickname?: string;
  avatarUrl?: string;
  bio?: string;
  totalXP: number;
  level?: number;
  xpIntoLevel?: number;
  xpToNextLevel?: number;
  levelProgress?: number;
  rankCode?: string;
  rankLabel?: string;
}

export interface BillingProduct {
  code: string;
  name: string;
  kind: "SUBSCRIPTION" | "LIFETIME";
  amountCents: number;
  creditAllowance: number;
  periodDays: number;
  adsFree: boolean;
  description: string;
}

export interface BillingStatus {
  productCode: string;
  productName: string;
  isLifetime: boolean;
  adsFree: boolean;
  creditBalance: number;
  creditAllowance: number;
  creditResetAt?: string;
  expiresAt?: string;
}

export interface AICreditCost {
  action: string;
  label: string;
  credits: number;
  description: string;
}

export interface PaymentOrder {
  id: string;
  productCode: string;
  amountCents: number;
  paymentChannel: string;
  status: "PENDING" | "PAID" | "CLOSED";
  checkoutURL?: string;
  createdAt?: string;
  paidAt?: string;
}

export interface AdPlacement {
  placement: "COURSES" | "LIBRARY" | "RESULT";
  provider: string;
  scriptURL?: string;
  slotId?: string;
}

export interface XPEvent {
  id: string;
  activity: string;
  sourceId: string;
  xpEarned: number;
  createdAt?: string;
}

export interface AuthResult {
  accessToken?: string;
  refreshToken?: string;
  userId?: string;
  emailVerificationRequired?: boolean;
  emailSent?: boolean;
  onboardingRequired?: boolean;
  message?: string;
}

export interface LoginCandidate {
  id: string;
  username: string;
  email: string;
}

export interface EmailActionResult {
  requiresSelection?: boolean;
  candidates?: LoginCandidate[];
  message?: string;
}

export interface ModelConfig {
  provider: string;
  model: string;
  baseURL: string;
  hasApiKey: boolean;
  apiKeyPreview: string;
  updatedAt?: string;
}

export interface TTSConfig {
  provider: string;
  model: string;
  baseURL: string;
  voice: string;
  audioFormat: string;
  hasApiKey: boolean;
  apiKeyPreview: string;
  updatedAt?: string;
}

export interface ASRConfig {
  provider: string;
  model: string;
  baseURL: string;
  hasApiKey: boolean;
  apiKeyPreview: string;
  appId?: string;
  updatedAt?: string;
}

export type VoiceProfileStatus = "GENERATING" | "PREVIEW" | "READY" | "FAILED";

export interface VoiceProfile {
  id: string;
  name: string;
  prompt: string;
  language: "CANTONESE" | "ENGLISH";
  provider: string;
  model: string;
  previewAudioUrl?: string;
  status: VoiceProfileStatus;
  generationMessage?: string;
  createdAt?: string;
}

export interface Dialogue {
  speaker: string;
  gender?: 'FEMALE' | 'MALE';
  text: string;
  zhSubtitle?: string;
  audioUrl: string;
  timestamp: number;
}

export interface TheaterQuizQuestion {
  question: string;
  options?: string[];
  answerKey?: string;
  type?: string;
  paragraphRef?: string;
  evidence?: string;
  headings?: string[];
  statements?: {
    id?: string;
    text: string;
    answer: string;
  }[];
  summaryText?: string;
  wordBank?: string[];
  answers?: string[];
}

export interface Character {
  name: string;
  role: string;
  color: string;
}

export interface Theater {
  id: string;
  language: "CANTONESE" | "ENGLISH";
  topic: string;
  difficulty: number;
  mode: "LISTENING" | "ROLEPLAY" | "APPRECIATION";
	status: TheaterStatus;
	generationProgress?: number;
	generationMessage?: string;
  isFavorite?: boolean;
  shareCode?: string;
  sceneDescription?: string;
  characters?: Character[];
  dialogues: Dialogue[];
  quizQuestions?: TheaterQuizQuestion[];
}

export type TheaterSummary = Omit<Theater, "characters" | "dialogues" | "quizQuestions">;

export interface PracticeResult {
  score: number;
  xpEarned: number;
  feedback: string;
  correctCount: number;
  totalCount: number;
}

export interface Course {
  id: string;
  language: "CANTONESE" | "ENGLISH";
  category: string;
  title: string;
  description: string;
  minLevel: number;
  maxLevel: number;
  isActive: boolean;
}

export interface RoleplaySession {
  id: string;
  theaterId: string;
  userRole: string;
  turnIndex: number;
  currentScore: number;
  status: string;
  processingMessage?: string;
  finalFeedback: string;
  transcript: Dialogue[];
}

export interface WritingPrompt {
  title: string;
  instructions: string;
  suggestedWordCount: number;
}

export interface WritingEvaluation {
  bandEstimate?: number | null;
  evidence?: string[] | null;
  overallScore: number;
  grammarScore: number;
  vocabularyScore: number;
  coherenceScore: number;
  taskResponseScore: number;
  strengths: string[];
  issues: string[];
  suggestions: string[];
  revisedExcerpt: string;
  summary: string;
}

export interface WritingSession {
  id: string;
  exam: "IELTS" | "CET4" | "CET6";
  timeLimitSeconds: number;
  prompt: WritingPrompt;
  essay: string;
  wordCount: number;
  status: "WRITING" | "EVALUATING" | "COMPLETED" | "FAILED";
  progressMessage: string;
  evaluation?: WritingEvaluation;
  startedAt: string;
  submittedAt?: string;
}

export interface SpeakingPrompt { part: number; question: string; cueCard: string; preparationSec: number; answerSec: number; questionId?: string; source?: string; audioUrl?: string; }
export interface SpeakingTurn { part: number; promptIndex: number; prompt: string; transcript: string; audioUrl?: string; examinerText?: string; examinerAudioUrl?: string; audioEvidence?: boolean; asrProvider?: string; asrModel?: string; }
export interface SpeakingEvaluation { textCoherence?: number | null; fluencyCoherence?: number | null; lexicalResource?: number | null; grammarAccuracy?: number | null; pronunciation?: number | null; overallBand?: number | null; isPartial?: boolean; assessmentMode?: string; strengths: string[]; improvements: string[]; evidence: string[]; summary: string; }
export interface SpeakingSession { id: string; status: string; part: number; promptIndex: number; pendingPromptIndex?: number; processingMessage?: string; lastError?: string; prompts: SpeakingPrompt[]; turns: SpeakingTurn[]; evaluation?: SpeakingEvaluation | null; }

export interface ListeningTrainingQuestion {
  question: string;
  type: string;
  options: string[];
  answerKey?: string | null;
  evidence?: string | null;
  correct?: boolean | null;
}

export interface ListeningTraining {
  id: string;
  part: number;
  status: "GENERATING" | "READY" | "IN_PROGRESS" | "COMPLETED" | "FAILED";
  message: string;
  title?: string | null;
  targetBand: number;
  createdAt: string;
  estimatedReadySeconds: number;
  generationEstimateSamples: number;
  instructions?: string | null;
  transcript?: string | null;
  audioUrls: string[];
  questions: ListeningTrainingQuestion[];
  answers: string[];
  correct?: number | null;
  total?: number | null;
  accuracy?: number | null;
  feedback?: string | null;
  recommendations?: string[] | null;
}

export interface MockExamQuestion {
  question: string;
  options: string[];
  type: string;
}

export interface MockExamSection {
  key: string;
  title: string;
  skill: "LISTENING" | "READING" | "WRITING" | "TRANSLATION";
  durationSeconds: number;
  generationDurationSeconds?: number | null;
  instructions: string;
  passage: string | null;
  audioUrl?: string | null;
  audioUrls?: (string | null)[] | null;
  questions: MockExamQuestion[] | null;
  writingPrompts: WritingPrompt[] | null;
  answers?: (string | null)[] | null;
  responses?: (string | null)[] | null;
}

export interface MockExamResult {
  scoreScale?: string;
  totalScore?: number;
  translationScore?: number;
  translationEvaluation?: WritingEvaluation | null;
  writingEvaluations?: WritingEvaluation[] | null;
  readingCorrect: number;
  readingTotal: number;
  listeningCorrect: number;
  listeningTotal: number;
  readingScore: number;
  listeningScore: number;
  writingScore: number;
  writingTask1Score: number;
  writingTask2Score: number;
  writingBand?: number | null;
  writingTask1Band?: number | null;
  writingTask2Band?: number | null;
  estimatedBand: number;
  qualityStatus: string;
  feedback: string;
  strengths: string[];
  weaknesses: string[];
  recommendations: string[];
  completedAt: string;
}

export interface MockExam {
  id: string;
  exam: string;
  status: "GENERATING" | "READY" | "IN_PROGRESS" | "EVALUATING" | "COMPLETED" | "FAILED" | "EVALUATION_FAILED";
  targetBand?: number | null;
  paperVersion?: string | null;
  currentSection: string;
  totalDurationSeconds: number;
  sections: MockExamSection[];
  result?: MockExamResult | null;
  startedAt: string;
  submittedAt?: string;
  estimatedReadySeconds?: number;
  generationEstimateSamples?: number;
  createdAt?: string;
}

export interface ContentSource {
  id: string;
  name: string;
  domain: string;
  category: string;
  exam: string;
  useCases: string[];
  contentMode: string;
  enabled: boolean;
  priority: number;
}

export interface ReadingMaterial {
  id: string;
  exam: string;
  language: string;
  level: string;
  topic: string;
  band?: number;
  stage?: string;
  section?: string;
  skillFocus?: string;
  questionType?: string;
  scenarioFamily?: string;
  title: string;
  passage: string;
  vocabulary: string[];
  questions: TheaterQuizQuestion[];
  sourceIds: string[];
  generationNote: string;
  audioUrl?: string;
  audioUrls?: string[];
	audioStatus?: "PENDING" | "READY" | "FAILED";
	status?: TheaterStatus;
	generationProgress?: number;
	generationMessage?: string;
  vocabularyItems?: {
    word: string;
    pos: string;
    meanings: string[];
  }[];
  associationSentences?: string[];
  grammarInsights?: {
    sentence: string;
    difficultyPoints: string[];
    studySuggestions: string[];
  }[];
}
