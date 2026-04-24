import type { ContentSource, Course, PracticeResult, ReadingMaterial, RoleplaySession, Theater, User } from "./types";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8177/graphql";

export function getApiBaseUrl(): string {
  try {
    return new URL(API_URL, window.location.origin).origin;
  } catch {
    return window.location.origin;
  }
}

type GraphQLResponse<T> = {
  data?: T;
  errors?: { message: string }[];
};

async function ensureAccessToken(): Promise<string> {
  const existing = localStorage.getItem("accessToken");
  if (existing) {
    return existing;
  }
  const seed = `${Date.now()}_${Math.random().toString(16).slice(2)}`;
  const email = `guest_${seed}@linguaquest.local`;
  const token = await register(email, "guest1234");
  localStorage.setItem("accessToken", token);
  return token;
}

async function request<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  async function sendRequest(token?: string | null): Promise<GraphQLResponse<T>> {
    const response = await fetch(API_URL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {})
      },
      body: JSON.stringify({ query, variables })
    });
    return response.json();
  }

  const currentToken = localStorage.getItem("accessToken");
  let result = await sendRequest(currentToken);

  // Token may be stale after backend JWT secret rotation; refresh once automatically.
  if (result.errors?.[0]?.message?.toLowerCase().includes("unauthorized") && currentToken) {
    localStorage.removeItem("accessToken");
    const renewedToken = await ensureAccessToken();
    result = await sendRequest(renewedToken);
  }

  if (result.errors?.length) {
    throw new Error(result.errors[0].message);
  }
  if (!result.data) {
    throw new Error("Empty response");
  }
  return result.data;
}

function isAnswerKeyFieldMissingError(err: unknown): boolean {
  const message = (err as Error)?.message ?? "";
  return message.includes('Cannot query field "answerKey" on type "TheaterQuizQuestion"');
}

function stripAnswerKeyField(query: string): string {
  return query.replace(/\s*answerKey\s*/g, " ");
}

async function requestWithAnswerKeyFallback<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  try {
    return await request<T>(query, variables);
  } catch (err) {
    if (!isAnswerKeyFieldMissingError(err)) {
      throw err;
    }
    return request<T>(stripAnswerKeyField(query), variables);
  }
}

export async function register(email: string, password: string): Promise<string> {
  const data = await request<{ register: { accessToken: string } }>(
    `mutation Register($email: String!, $password: String!) {
      register(email: $email, password: $password) { accessToken }
    }`,
    { email, password }
  );
  return data.register.accessToken;
}

export async function login(email: string, password: string): Promise<string> {
  const data = await request<{ login: { accessToken: string } }>(
    `mutation Login($email: String!, $password: String!) {
      login(email: $email, password: $password) { accessToken }
    }`,
    { email, password }
  );
  return data.login.accessToken;
}

export async function me(): Promise<User> {
  const data = await request<{ me: User }>(
    `query Me { me { id email nickname avatarUrl bio totalXP } }`
  );
  return data.me;
}

export async function updateProfile(input: {
  nickname?: string;
  avatarUrl?: string;
  bio?: string;
}): Promise<User> {
  const data = await request<{ updateProfile: User }>(
    `mutation UpdateProfile($nickname: String, $avatarUrl: String, $bio: String) {
      updateProfile(nickname: $nickname, avatarUrl: $avatarUrl, bio: $bio) { id email nickname avatarUrl bio totalXP }
    }`,
    input
  );
  return data.updateProfile;
}

export async function generateTheater(input: {
  language: "CANTONESE" | "ENGLISH";
  topic: string;
  difficulty: number;
  mode: "LISTENING" | "ROLEPLAY" | "APPRECIATION";
}): Promise<Theater> {
  await ensureAccessToken();
  const data = await request<{ generateTheater: Theater }>(
    `mutation Generate($input: GenerateTheaterInput!) {
      generateTheater(input: $input) { id language topic difficulty mode status isFavorite shareCode sceneDescription characters { name role color } dialogues { speaker text zhSubtitle audioUrl timestamp } quizQuestions { question options } }
    }`,
    { input }
  );
  return data.generateTheater;
}

export async function getTheater(id: string): Promise<Theater> {
  const data = await request<{ theater: Theater }>(
    `query Theater($id: ID!) { theater(id: $id) { id language topic difficulty mode status isFavorite shareCode sceneDescription characters { name role color } dialogues { speaker text zhSubtitle audioUrl timestamp } quizQuestions { question options } } }`,
    { id }
  );
  return data.theater;
}

export async function getSharedTheater(shareCode: string): Promise<Theater> {
  const data = await request<{ sharedTheater: Theater }>(
    `query SharedTheater($shareCode: String!) {
      sharedTheater(shareCode: $shareCode) {
        id language topic difficulty mode status isFavorite shareCode sceneDescription
        characters { name role color }
        dialogues { speaker text zhSubtitle audioUrl timestamp }
        quizQuestions { question options }
      }
    }`,
    { shareCode }
  );
  return data.sharedTheater;
}

export async function submitAnswers(theaterId: string, answers: string[]): Promise<PracticeResult> {
  const data = await request<{ submitAnswers: PracticeResult }>(
    `mutation Submit($theaterId: ID!, $answers: [String!]!) {
      submitAnswers(theaterId: $theaterId, answers: $answers) { score xpEarned feedback correctCount totalCount }
    }`,
    { theaterId, answers }
  );
  return data.submitAnswers;
}

export async function myTheaters(filter?: {
  language?: "CANTONESE" | "ENGLISH";
  status?: "GENERATING" | "READY" | "FAILED";
  favorite?: boolean;
}): Promise<Theater[]> {
  await ensureAccessToken();
  const data = await request<{ myTheaters: Theater[] }>(
    `query MyTheaters($language: String, $status: String, $favorite: Boolean) {
      myTheaters(language: $language, status: $status, favorite: $favorite) {
        id language topic difficulty mode status isFavorite shareCode sceneDescription characters { name role color } dialogues { speaker text zhSubtitle audioUrl timestamp } quizQuestions { question options }
      }
    }`,
    filter
  );
  return data.myTheaters;
}

export async function toggleFavorite(theaterId: string, favorite: boolean): Promise<boolean> {
  await ensureAccessToken();
  const data = await request<{ toggleFavorite: boolean }>(
    `mutation ToggleFavorite($theaterId: ID!, $favorite: Boolean!) {
      toggleFavorite(theaterId: $theaterId, favorite: $favorite)
    }`,
    { theaterId, favorite }
  );
  return data.toggleFavorite;
}

export async function shareTheater(theaterId: string): Promise<string> {
  await ensureAccessToken();
  const data = await request<{ shareTheater: string }>(
    `mutation Share($theaterId: ID!) { shareTheater(theaterId: $theaterId) }`,
    { theaterId }
  );
  return data.shareTheater;
}

export async function deleteTheater(theaterId: string): Promise<boolean> {
  await ensureAccessToken();
  const data = await request<{ deleteTheater: boolean }>(
    `mutation DeleteTheater($theaterId: ID!) {
      deleteTheater(theaterId: $theaterId)
    }`,
    { theaterId }
  );
  return data.deleteTheater;
}

export async function courses(language?: "CANTONESE" | "ENGLISH"): Promise<Course[]> {
  const data = await request<{ courses: Course[] }>(
    `query Courses($language: String) {
      courses(language: $language) { id language category title description minLevel maxLevel isActive }
    }`,
    { language }
  );
  return data.courses;
}

export async function startRoleplay(theaterId: string, userRole: string): Promise<RoleplaySession> {
  await ensureAccessToken();
  const data = await request<{ startRoleplay: RoleplaySession }>(
    `mutation StartRoleplay($theaterId: ID!, $userRole: String!) {
      startRoleplay(theaterId: $theaterId, userRole: $userRole) {
        id theaterId userRole turnIndex currentScore status finalFeedback transcript { speaker text zhSubtitle audioUrl timestamp }
      }
    }`,
    { theaterId, userRole }
  );
  return data.startRoleplay;
}

export async function submitRoleplayReply(sessionId: string, text: string): Promise<RoleplaySession> {
  await ensureAccessToken();
  const data = await request<{ submitRoleplayReply: RoleplaySession }>(
    `mutation SubmitRoleplay($sessionId: ID!, $text: String!) {
      submitRoleplayReply(sessionId: $sessionId, text: $text) {
        id theaterId userRole turnIndex currentScore status finalFeedback transcript { speaker text zhSubtitle audioUrl timestamp }
      }
    }`,
    { sessionId, text }
  );
  return data.submitRoleplayReply;
}

export async function endRoleplay(sessionId: string): Promise<RoleplaySession> {
  await ensureAccessToken();
  const data = await request<{ endRoleplay: RoleplaySession }>(
    `mutation EndRoleplay($sessionId: ID!) {
      endRoleplay(sessionId: $sessionId) {
        id theaterId userRole turnIndex currentScore status finalFeedback transcript { speaker text zhSubtitle audioUrl timestamp }
      }
    }`,
    { sessionId }
  );
  return data.endRoleplay;
}

export async function contentSources(filter?: { exam?: string; category?: string }): Promise<ContentSource[]> {
  const data = await request<{ contentSources: ContentSource[] }>(
    `query ContentSources($exam: String, $category: String) {
      contentSources(exam: $exam, category: $category) {
        id name domain category exam useCases contentMode enabled priority
      }
    }`,
    filter
  );
  return data.contentSources;
}

export async function generateReading(input: {
  exam: string;
  topic: string;
  level?: string;
  sourceIds?: string[];
}): Promise<ReadingMaterial> {
  await ensureAccessToken();
  const query = `mutation GenerateReading($exam: String!, $topic: String!, $level: String, $sourceIds: [String!]) {
      generateReading(exam: $exam, topic: $topic, level: $level, sourceIds: $sourceIds) {
        id exam language level topic title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ generateReading: ReadingMaterial }>(query, input);
  return data.generateReading;
}

export async function readingMaterials(exam?: string): Promise<ReadingMaterial[]> {
  await ensureAccessToken();
  const query = `query ReadingMaterials($exam: String) {
      readingMaterials(exam: $exam) {
        id exam language level topic title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ readingMaterials: ReadingMaterial[] }>(query, { exam });
  return data.readingMaterials;
}

export async function readingMaterial(id: string): Promise<ReadingMaterial> {
  await ensureAccessToken();
  const query = `query ReadingMaterial($id: ID!) {
      readingMaterial(id: $id) {
        id exam language level topic title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ readingMaterial: ReadingMaterial }>(query, { id });
  return data.readingMaterial;
}

export async function submitReadingAnswers(materialId: string, answers: string[]): Promise<PracticeResult> {
  await ensureAccessToken();
  const data = await request<{ submitReadingAnswers: PracticeResult }>(
    `mutation SubmitReadingAnswers($materialId: ID!, $answers: [String!]!) {
      submitReadingAnswers(materialId: $materialId, answers: $answers) {
        score xpEarned feedback correctCount totalCount
      }
    }`,
    { materialId, answers }
  );
  return data.submitReadingAnswers;
}
