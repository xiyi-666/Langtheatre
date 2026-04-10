import type { Course, PracticeResult, RoleplaySession, Theater, User } from "./types";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8177/graphql";

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
