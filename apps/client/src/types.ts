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

export type VoiceProfileStatus = "GENERATING" | "READY" | "FAILED";

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
  status: "WRITING" | "EVALUATING" | "COMPLETED";
  progressMessage: string;
  evaluation?: WritingEvaluation;
  startedAt: string;
  submittedAt?: string;
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
