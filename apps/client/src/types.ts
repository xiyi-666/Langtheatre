export type TheaterStatus = "GENERATING" | "READY" | "FAILED";

export interface User {
  id: string;
  email: string;
  nickname?: string;
  avatarUrl?: string;
  bio?: string;
  totalXP: number;
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
  isFavorite?: boolean;
  shareCode?: string;
  sceneDescription?: string;
  characters?: Character[];
  dialogues: Dialogue[];
  quizQuestions?: TheaterQuizQuestion[];
}

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
  finalFeedback: string;
  transcript: Dialogue[];
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
  title: string;
  passage: string;
  vocabulary: string[];
  questions: TheaterQuizQuestion[];
  sourceIds: string[];
  generationNote: string;
  audioUrl?: string;
  audioUrls?: string[];
  audioStatus?: "PENDING" | "READY" | "FAILED";
}
