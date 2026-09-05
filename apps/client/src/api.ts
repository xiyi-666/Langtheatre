import type { AICreditCost, ASRConfig, AdPlacement, AuthResult, BillingProduct, BillingStatus, ContentSource, Course, EmailActionResult, LoginCandidate, ModelConfig, PaymentOrder, PracticeResult, ReadingMaterial, RoleplaySession, TTSConfig, Theater, TheaterSummary, User, VoiceProfile, WritingSession, XPEvent } from "./types";

export const DESKTOP_API_CONFIGURATION_ERROR = "桌面端 API 未配置，请联系管理员重新构建应用。";

export function resolveApiUrl(
	envUrl = (import.meta.env.VITE_API_URL as string | undefined)?.trim(),
	isTauriRuntime = typeof window !== "undefined" && (window.location.protocol === "tauri:" || window.location.hostname === "tauri.localhost" || window.location.hostname.endsWith(".localhost"))
): string | undefined {
  if (envUrl) {
    return envUrl;
  }

  if (isTauriRuntime) {
		return undefined;
  }

  if (typeof window === "undefined") {
    return "/graphql";
  }

  return "/graphql";
}

const API_URL = resolveApiUrl();
export const CREDIT_INSUFFICIENT_EVENT = "linguaquest:credit-insufficient";

function configuredApiUrl(): string {
	if (!API_URL) {
		throw new Error(DESKTOP_API_CONFIGURATION_ERROR);
	}
	return API_URL;
}

function resolveTelemetryUrl(): string | undefined {
	if (!API_URL) {
		return undefined;
	}
	try {
		const apiURL = new URL(API_URL, window.location.origin);
		return new URL("/telemetry/event", apiURL).toString();
	} catch {
		return "/telemetry/event";
	}
}

const TELEMETRY_URL = resolveTelemetryUrl();

export function isAICreditInsufficientError(error: unknown): boolean {
	const message = error instanceof Error ? error.message : String(error ?? "");
	return message.includes("AI 点数不足");
}

function notifyAICreditInsufficient(message: string): void {
	if (typeof window === "undefined") {
		return;
	}
	window.dispatchEvent(new CustomEvent(CREDIT_INSUFFICIENT_EVENT, { detail: { message } }));
}

// Telemetry is deliberately fire-and-forget: it stores only a daily aggregate
// event name and never includes user content, identifiers, or request payloads.
export function trackClick(name: string): void {
	const token = localStorage.getItem("accessToken");
	if (!TELEMETRY_URL || !token || !/^[A-Z][A-Z0-9_]{2,63}$/.test(name)) {
		return;
	}
	void fetch(TELEMETRY_URL, {
		method: "POST",
		keepalive: true,
		headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
		body: JSON.stringify({ category: "CLICK", name })
	}).catch(() => undefined);
}

export function getApiBaseUrl(): string {
	const apiUrl = configuredApiUrl();
  try {
    return new URL(apiUrl, window.location.origin).origin;
  } catch {
    return window.location.origin;
  }
}

type GraphQLResponse<T> = {
  data?: T;
  errors?: { message: string }[];
};

export function localizeErrorMessage(message: string): string {
  const original = String(message ?? "").trim();
  if (!original) {
    return "服务暂时无法处理请求，请稍后重试。";
  }
  if (/[\u4e00-\u9fff]/.test(original)) {
    return original;
  }
  const normalized = original.toLowerCase();
  if (normalized.includes("failed to fetch") || normalized.includes("networkerror") || normalized.includes("network error")) {
    return "无法连接服务器，请检查网络连接或稍后重试。";
  }
  if (normalized.includes("invalid credentials")) {
    return "用户名或密码错误。";
  }
  if (normalized.includes("authentication rate limit") || normalized.includes("too many login") || normalized.includes("too many requests")) {
    return "操作过于频繁，请 1 分钟后再试。";
  }
  if (normalized.includes("ai request rate limit")) {
    return "AI 请求过于频繁，请稍后再试。";
  }
  if (normalized.includes("unauthorized")) {
    return "未授权，请重新登录。";
  }
  if (normalized.includes("refresh token invalid") || normalized.includes("refresh token is invalid")) {
    return "登录状态已失效，请重新登录。";
  }
  if (normalized.includes("refresh is unavailable")) {
    return "登录服务暂不可用，请稍后重试。";
  }
  if (normalized.includes("session is unavailable")) {
    return "登录会话暂不可用，请稍后重试。";
  }
  if (normalized.includes("invalid email")) {
    return "邮箱格式不正确，请检查后重试。";
  }
  if (normalized.includes("password must")) {
    return "密码须为 8–15 位，并同时包含大写字母、小写字母和数字。";
  }
  if (normalized.includes("username must")) {
    return "用户名须为 3–24 位，只能包含字母、数字、下划线或短横线。";
  }
  if (normalized.includes("username already exists")) {
    return "用户名已存在，请换一个用户名。";
  }
  if (normalized.includes("email account limit reached")) {
    return "该邮箱最多注册 3 个账号，已达到上限。";
  }
  if (normalized.includes("multiple accounts found")) {
    return "该邮箱关联多个账号，请选择要登录的账号。";
  }
  if (normalized.includes("invalid account selection")) {
    return "账号选择无效，请重新选择。";
  }
  if (normalized.includes("smtp is not configured")) {
    return "邮箱服务暂不可用，请联系管理员配置 SMTP。";
  }
  if (normalized.includes("password reset email sent")) {
    return "密码重置邮件已发送。";
  }
  if (normalized.includes("username recovery email sent")) {
    return "用户名找回邮件已发送。";
  }
  if (normalized.includes("empty response")) {
    return "服务器返回了空结果，请稍后重试。";
  }
  if (normalized.includes("cannot query field") || normalized.includes("not supported")) {
    return "当前服务版本暂不支持此功能，请联系管理员更新服务。";
  }
  if (normalized.includes("is required") || normalized.includes("required")) {
    return "请填写完整信息后再提交。";
  }
  return "服务暂时无法处理请求，请稍后重试。";
}

function clearStoredSession(): void {
	localStorage.removeItem("accessToken");
	localStorage.removeItem("refreshToken");
}

function isUnauthorized(response: GraphQLResponse<unknown>): boolean {
	const message = response.errors?.[0]?.message ?? "";
	return message.toLowerCase().includes("unauthorized") || message.includes("未授权") || message.includes("登录状态已失效");
}

async function sendRequest<T>(query: string, variables?: Record<string, unknown>, token?: string | null): Promise<GraphQLResponse<T>> {
	let response: Response;
	try {
		response = await fetch(configuredApiUrl(), {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
				...(token ? { Authorization: `Bearer ${token}` } : {})
			},
			body: JSON.stringify({ query, variables })
		});
	} catch (error) {
		throw new Error(localizeErrorMessage(error instanceof Error ? error.message : String(error)));
	}
	if (!response.ok) {
		const fallback = response.status === 429 ? "操作过于频繁，请稍后再试。" : "服务暂时无法处理请求，请稍后重试。";
		const message = (await response.text()).trim();
		let extracted = message;
		try {
			const payload = JSON.parse(message) as GraphQLResponse<unknown>;
			extracted = payload.errors?.[0]?.message || message;
		} catch {
			// 非 JSON 响应直接按文本处理。
		}
		throw new Error(localizeErrorMessage(extracted || fallback));
	}
	try {
		return await response.json();
	} catch {
		throw new Error("服务器返回数据格式错误，请稍后重试。");
	}
}

async function refreshAccessToken(refreshToken: string): Promise<{ accessToken: string; refreshToken: string } | undefined> {
	try {
		const result = await sendRequest<{ refresh: { accessToken?: string; refreshToken?: string } }>(
			"mutation Refresh($refreshToken: String!) { refresh(refreshToken: $refreshToken) { accessToken refreshToken } }",
			{ refreshToken }
		);
		const refreshed = result.data?.refresh;
		if (result.errors?.length || !refreshed?.accessToken || !refreshed.refreshToken) {
			return undefined;
		}
		return { accessToken: refreshed.accessToken, refreshToken: refreshed.refreshToken };
	} catch {
		return undefined;
	}
}

async function ensureAccessToken(): Promise<string> {
  const existing = localStorage.getItem("accessToken");
  if (existing) {
    return existing;
  }
  throw new Error("请先登录后再继续操作");
}

async function request<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  const currentToken = localStorage.getItem("accessToken");
  let result = await sendRequest<T>(query, variables, currentToken);

  if (isUnauthorized(result) && currentToken) {
		const refreshToken = localStorage.getItem("refreshToken");
		const refreshed = refreshToken ? await refreshAccessToken(refreshToken) : undefined;
		if (!refreshed) {
			clearStoredSession();
			throw new Error("登录状态已失效，请重新登录");
		}
		localStorage.setItem("accessToken", refreshed.accessToken);
		localStorage.setItem("refreshToken", refreshed.refreshToken);
		result = await sendRequest<T>(query, variables, refreshed.accessToken);
		if (isUnauthorized(result)) {
			clearStoredSession();
			throw new Error("登录状态已失效，请重新登录");
  }
  }

	if (result.errors?.length) {
		const rawMessage = result.errors[0].message;
		const message = localizeErrorMessage(rawMessage);
		if (isAICreditInsufficientError(new Error(message))) {
			notifyAICreditInsufficient(message);
		}
		throw new Error(message);
  }
  if (!result.data) {
		throw new Error("服务器返回了空结果，请稍后重试。");
  }
  return result.data;
}

async function requestAuthResult<T extends Record<string, AuthResult>>(
	_operation: string,
	query: string,
	fallbackQuery: string,
	variables: Record<string, unknown>
): Promise<T> {
	try {
		return await request<T>(query, variables);
	} catch (error) {
		// 线上兼容后端可能尚未发布 onboardingRequired 字段，自动降级一次。
		if (error instanceof Error && error.message.includes("当前服务版本暂不支持")) {
			return request<T>(fallbackQuery, variables);
		}
		throw error;
	}
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

export async function register(username: string, email: string, password: string): Promise<AuthResult> {
	const data = await requestAuthResult("register", "mutation Register($username: String!, $email: String!, $password: String!) { register(username: $username, email: $email, password: $password) { accessToken refreshToken userId emailVerificationRequired emailSent onboardingRequired message } }", "mutation Register($username: String!, $email: String!, $password: String!) { register(username: $username, email: $email, password: $password) { accessToken refreshToken userId emailVerificationRequired emailSent message } }", { username, email, password });
	return data.register;
}

export async function login(identifier: string, password: string, userId?: string): Promise<AuthResult> {
	const data = await requestAuthResult("login", "mutation Login($identifier: String!, $password: String!, $userId: String) { login(identifier: $identifier, password: $password, userId: $userId) { accessToken refreshToken userId emailVerificationRequired emailSent onboardingRequired message } }", "mutation Login($identifier: String!, $password: String!, $userId: String) { login(identifier: $identifier, password: $password, userId: $userId) { accessToken refreshToken userId emailVerificationRequired emailSent message } }", { identifier, password, userId });
	return data.login;
}

export async function logout(): Promise<boolean> {
	const data = await request<{ logout: boolean }>(`mutation Logout { logout }`);
	return data.logout;
}

export async function loginCandidates(identifier: string): Promise<LoginCandidate[]> {
	const data = await request<{ loginCandidates: LoginCandidate[] }>(
		`query LoginCandidates($identifier: String!) { loginCandidates(identifier: $identifier) { id username email } }`,
		{ identifier }
	);
	return data.loginCandidates;
}

export async function requestEmailVerification(identifier: string, userId?: string): Promise<EmailActionResult> {
	const data = await request<{ requestEmailVerification: EmailActionResult }>(
		`mutation RequestEmailVerification($identifier: String!, $userId: String) { requestEmailVerification(identifier: $identifier, userId: $userId) { requiresSelection candidates { id username email } message } }`,
		{ identifier, userId }
	);
	return data.requestEmailVerification;
}

export async function verifyEmail(token: string): Promise<AuthResult> {
	const data = await requestAuthResult("verifyEmail", "mutation VerifyEmail($token: String!) { verifyEmail(token: $token) { accessToken refreshToken userId onboardingRequired message } }", "mutation VerifyEmail($token: String!) { verifyEmail(token: $token) { accessToken refreshToken userId message } }", { token });
	return data.verifyEmail;
}

export async function requestPasswordReset(identifier: string, userId?: string): Promise<EmailActionResult> {
	const data = await request<{ requestPasswordReset: EmailActionResult }>(
		`mutation RequestPasswordReset($identifier: String!, $userId: String) { requestPasswordReset(identifier: $identifier, userId: $userId) { requiresSelection candidates { id username email } message } }`,
		{ identifier, userId }
	);
	return data.requestPasswordReset;
}

export async function resetPassword(token: string, password: string): Promise<boolean> {
	const data = await request<{ resetPassword: boolean }>(
		`mutation ResetPassword($token: String!, $password: String!) { resetPassword(token: $token, password: $password) }`,
		{ token, password }
	);
	return data.resetPassword;
}

export async function requestUsernameRecovery(email: string): Promise<boolean> {
	const data = await request<{ requestUsernameRecovery: boolean }>(
		`mutation RequestUsernameRecovery($email: String!) { requestUsernameRecovery(email: $email) }`,
		{ email }
	);
	return data.requestUsernameRecovery;
}

export async function me(): Promise<User> {
  const data = await request<{ me: User }>(
		`query Me { me { id username email emailVerified nickname avatarUrl bio totalXP level xpIntoLevel xpToNextLevel levelProgress rankCode rankLabel } }`
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
		updateProfile(nickname: $nickname, avatarUrl: $avatarUrl, bio: $bio) { id username email emailVerified nickname avatarUrl bio totalXP level xpIntoLevel xpToNextLevel levelProgress rankCode rankLabel }
    }`,
    input
  );
  return data.updateProfile;
}

export async function billingProducts(): Promise<BillingProduct[]> {
  const data = await request<{ billingProducts: BillingProduct[] }>(`query BillingProducts { billingProducts { code name kind amountCents creditAllowance periodDays adsFree description } }`);
  return data.billingProducts;
}

export async function billingStatus(): Promise<BillingStatus> {
  await ensureAccessToken();
  const data = await request<{ billingStatus: BillingStatus }>(`query BillingStatus { billingStatus { productCode productName isLifetime adsFree creditBalance creditAllowance creditResetAt expiresAt } }`);
  return data.billingStatus;
}

export async function createPaymentOrder(productCode: string, channel: "alipay" | "wxpay" = "alipay"): Promise<PaymentOrder> {
	await ensureAccessToken();
	const data = await request<{ createPaymentOrder: PaymentOrder }>(`mutation CreatePaymentOrder($productCode: String!, $channel: String) { createPaymentOrder(productCode: $productCode, channel: $channel) { id productCode amountCents paymentChannel status checkoutURL createdAt paidAt } }`, { productCode, channel });
	return data.createPaymentOrder;
}

export async function paymentOrder(id: string): Promise<PaymentOrder> {
  await ensureAccessToken();
  const data = await request<{ paymentOrder: PaymentOrder }>(`query PaymentOrder($id: ID!) { paymentOrder(id: $id) { id productCode amountCents paymentChannel status checkoutURL createdAt paidAt } }`, { id });
  return data.paymentOrder;
}

export async function aiCreditCosts(): Promise<AICreditCost[]> {
  const data = await request<{ aiCreditCosts: AICreditCost[] }>(`query AICreditCosts { aiCreditCosts { action label credits description } }`);
  return data.aiCreditCosts;
}

export async function xpEvents(limit = 8): Promise<XPEvent[]> {
  await ensureAccessToken();
  const data = await request<{ xpEvents: XPEvent[] }>(`query XPEvents($limit: Int) { xpEvents(limit: $limit) { id activity sourceId xpEarned createdAt } }`, { limit });
  return data.xpEvents;
}

export async function adPlacements(): Promise<AdPlacement[]> {
  await ensureAccessToken();
  const data = await request<{ adPlacements: AdPlacement[] }>(`query AdPlacements { adPlacements { placement provider scriptURL slotId } }`);
  return data.adPlacements;
}

export async function getModelConfig(): Promise<ModelConfig> {
  await ensureAccessToken();
  const data = await request<{ modelConfig: ModelConfig }>(
    `query ModelConfig {
      modelConfig { provider model baseURL hasApiKey apiKeyPreview updatedAt }
    }`
  );
  return data.modelConfig;
}

export async function updateModelConfig(input: {
  provider?: string;
  model?: string;
  baseURL?: string;
  apiKey?: string;
}): Promise<ModelConfig> {
  await ensureAccessToken();
  const data = await request<{ updateModelConfig: ModelConfig }>(
    `mutation UpdateModelConfig($provider: String, $model: String, $baseURL: String, $apiKey: String) {
      updateModelConfig(provider: $provider, model: $model, baseURL: $baseURL, apiKey: $apiKey) {
        provider model baseURL hasApiKey apiKeyPreview updatedAt
      }
    }`,
    input
  );
  return data.updateModelConfig;
}

export async function getTTSConfig(): Promise<TTSConfig> {
  await ensureAccessToken();
  const data = await request<{ ttsConfig: TTSConfig }>(
    `query TTSConfig {
      ttsConfig { provider model baseURL voice audioFormat hasApiKey apiKeyPreview updatedAt }
    }`
  );
  return data.ttsConfig;
}

export async function updateTTSConfig(input: {
  provider?: string;
  model?: string;
  baseURL?: string;
  voice?: string;
  apiKey?: string;
}): Promise<TTSConfig> {
  await ensureAccessToken();
  const data = await request<{ updateTTSConfig: TTSConfig }>(
    `mutation UpdateTTSConfig($provider: String, $model: String, $baseURL: String, $voice: String, $apiKey: String) {
      updateTTSConfig(provider: $provider, model: $model, baseURL: $baseURL, voice: $voice, apiKey: $apiKey) {
        provider model baseURL voice audioFormat hasApiKey apiKeyPreview updatedAt
      }
    }`,
    input
  );
  return data.updateTTSConfig;
}

export async function getASRConfig(): Promise<ASRConfig> {
  await ensureAccessToken();
  const data = await request<{ asrConfig: ASRConfig }>(`query ASRConfig { asrConfig { provider model baseURL hasApiKey apiKeyPreview appId updatedAt } }`);
  return data.asrConfig;
}

export async function updateASRConfig(input: { provider?: string; model?: string; baseURL?: string; apiKey?: string; appId?: string }): Promise<ASRConfig> {
  await ensureAccessToken();
  const data = await request<{ updateASRConfig: ASRConfig }>(
    `mutation UpdateASRConfig($provider: String, $model: String, $baseURL: String, $apiKey: String, $appId: String) {
      updateASRConfig(provider: $provider, model: $model, baseURL: $baseURL, apiKey: $apiKey, appId: $appId) { provider model baseURL hasApiKey apiKeyPreview appId updatedAt }
    }`, input);
  return data.updateASRConfig;
}

export async function getVoiceProfiles(): Promise<VoiceProfile[]> {
  await ensureAccessToken();
  const data = await request<{ voiceProfiles: VoiceProfile[] }>(
    `query VoiceProfiles {
      voiceProfiles { id name prompt language provider model previewAudioUrl status generationMessage createdAt }
    }`
  );
  return data.voiceProfiles;
}

export async function createVoiceProfile(input: {
  name: string;
  prompt: string;
  language: "CANTONESE" | "ENGLISH";
}): Promise<VoiceProfile> {
  await ensureAccessToken();
  const data = await request<{ createVoiceProfile: VoiceProfile }>(
    `mutation CreateVoiceProfile($name: String!, $prompt: String!, $language: String!) {
      createVoiceProfile(name: $name, prompt: $prompt, language: $language) {
        id name prompt language provider model previewAudioUrl status generationMessage createdAt
      }
    }`,
    input
  );
  return data.createVoiceProfile;
}

export async function approveVoiceProfile(id: string): Promise<VoiceProfile> {
  await ensureAccessToken();
  const data = await request<{ approveVoiceProfile: VoiceProfile }>(
    `mutation ApproveVoiceProfile($id: ID!) {
      approveVoiceProfile(id: $id) {
        id name prompt language provider model previewAudioUrl status generationMessage createdAt
      }
    }`,
    { id }
  );
  return data.approveVoiceProfile;
}

export async function deleteVoiceProfile(id: string): Promise<void> {
  await ensureAccessToken();
  await request<{ deleteVoiceProfile: boolean }>(
    `mutation DeleteVoiceProfile($id: ID!) { deleteVoiceProfile(id: $id) }`,
    { id }
  );
}

export async function generateTheater(input: {
  language: "CANTONESE" | "ENGLISH";
  topic: string;
  difficulty: number;
  mode: "LISTENING" | "ROLEPLAY" | "APPRECIATION";
  voiceMode?: "AUTO" | "LIBRARY";
  voiceProfileIds?: string[];
}): Promise<Theater> {
  await ensureAccessToken();
  const data = await request<{ generateTheater: Theater }>(
    `mutation Generate($input: GenerateTheaterInput!) {
		generateTheater(input: $input) { id language topic difficulty mode status generationProgress generationMessage isFavorite shareCode sceneDescription characters { name role color } dialogues { speaker gender text zhSubtitle audioUrl timestamp } quizQuestions { question options } }
    }`,
    { input }
  );
  return data.generateTheater;
}

export async function getTheater(id: string): Promise<Theater> {
  const data = await request<{ theater: Theater }>(
		`query Theater($id: ID!) { theater(id: $id) { id language topic difficulty mode status generationProgress generationMessage isFavorite shareCode sceneDescription characters { name role color } dialogues { speaker gender text zhSubtitle audioUrl timestamp } quizQuestions { question options } } }`,
    { id }
  );
  return data.theater;
}

export async function getSharedTheater(shareCode: string): Promise<Theater> {
  const data = await request<{ sharedTheater: Theater }>(
    `query SharedTheater($shareCode: String!) {
      sharedTheater(shareCode: $shareCode) {
		id language topic difficulty mode status generationProgress generationMessage isFavorite shareCode sceneDescription
        characters { name role color }
		dialogues { speaker gender text zhSubtitle audioUrl timestamp }
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
}): Promise<TheaterSummary[]> {
  await ensureAccessToken();
  const data = await request<{ myTheaters: TheaterSummary[] }>(
    `query MyTheaters($language: String, $status: String, $favorite: Boolean) {
      myTheaters(language: $language, status: $status, favorite: $favorite) {
		id language topic difficulty mode status generationProgress generationMessage isFavorite shareCode sceneDescription
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
		id theaterId userRole turnIndex currentScore status processingMessage finalFeedback transcript { speaker gender text zhSubtitle audioUrl timestamp }
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
		id theaterId userRole turnIndex currentScore status processingMessage finalFeedback transcript { speaker gender text zhSubtitle audioUrl timestamp }
      }
    }`,
    { sessionId, text }
  );
  return data.submitRoleplayReply;
}

export async function submitRoleplayAudio(sessionId: string, audioDataUrl: string, language: string): Promise<RoleplaySession> {
  await ensureAccessToken();
  const data = await request<{ submitRoleplayAudio: RoleplaySession }>(
    `mutation SubmitRoleplayAudio($sessionId: ID!, $audioDataUrl: String!, $language: String!) {
      submitRoleplayAudio(sessionId: $sessionId, audioDataUrl: $audioDataUrl, language: $language) {
		id theaterId userRole turnIndex currentScore status processingMessage finalFeedback transcript { speaker gender text zhSubtitle audioUrl timestamp }
      }
    }`, { sessionId, audioDataUrl, language });
  return data.submitRoleplayAudio;
}

export async function getRoleplaySession(sessionId: string): Promise<RoleplaySession> {
  await ensureAccessToken();
	const data = await request<{ roleplaySession: RoleplaySession }>(`query RoleplaySession($sessionId: ID!) { roleplaySession(sessionId: $sessionId) { id theaterId userRole turnIndex currentScore status processingMessage finalFeedback transcript { speaker gender text zhSubtitle audioUrl timestamp } } }`, { sessionId });
  return data.roleplaySession;
}

export async function endRoleplay(sessionId: string): Promise<RoleplaySession> {
  await ensureAccessToken();
  const data = await request<{ endRoleplay: RoleplaySession }>(
    `mutation EndRoleplay($sessionId: ID!) {
      endRoleplay(sessionId: $sessionId) {
		id theaterId userRole turnIndex currentScore status processingMessage finalFeedback transcript { speaker gender text zhSubtitle audioUrl timestamp }
      }
    }`,
    { sessionId }
  );
  return data.endRoleplay;
}

const WRITING_SESSION_FIELDS = `id exam timeLimitSeconds prompt { title instructions suggestedWordCount } essay wordCount status progressMessage evaluation { overallScore grammarScore vocabularyScore coherenceScore taskResponseScore strengths issues suggestions revisedExcerpt summary } startedAt submittedAt`;

export async function startWritingSession(exam: "IELTS" | "CET4" | "CET6", timeLimitSeconds: number, difficulty?: number): Promise<WritingSession> {
  await ensureAccessToken();
  const data = await request<{ startWritingSession: WritingSession }>(`mutation StartWriting($exam: String!, $timeLimitSeconds: Int!, $difficulty: Float) { startWritingSession(exam: $exam, timeLimitSeconds: $timeLimitSeconds, difficulty: $difficulty) { ${WRITING_SESSION_FIELDS} } }`, { exam, timeLimitSeconds, difficulty });
  return data.startWritingSession;
}

export async function submitWritingSession(sessionId: string, essay: string): Promise<WritingSession> {
  await ensureAccessToken();
  const data = await request<{ submitWritingSession: WritingSession }>(`mutation SubmitWriting($sessionId: ID!, $essay: String!) { submitWritingSession(sessionId: $sessionId, essay: $essay) { ${WRITING_SESSION_FIELDS} } }`, { sessionId, essay });
  return data.submitWritingSession;
}

export async function getWritingSession(sessionId: string): Promise<WritingSession> {
  await ensureAccessToken();
  const data = await request<{ writingSession: WritingSession }>(`query WritingSession($sessionId: ID!) { writingSession(sessionId: $sessionId) { ${WRITING_SESSION_FIELDS} } }`, { sessionId });
  return data.writingSession;
}

export async function listWritingSessions(): Promise<WritingSession[]> {
  await ensureAccessToken();
  const data = await request<{ writingSessions: WritingSession[] }>(`query WritingSessions { writingSessions { ${WRITING_SESSION_FIELDS} } }`);
	return data.writingSessions;
}

export async function deleteWritingSession(sessionId: string): Promise<boolean> {
  await ensureAccessToken();
  const data = await request<{ deleteWritingSession: boolean }>(
    `mutation DeleteWritingSession($sessionId: ID!) { deleteWritingSession(sessionId: $sessionId) }`,
    { sessionId }
  );
  return data.deleteWritingSession;
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
  band?: number;
  stage?: string;
  section?: string;
  skillFocus?: string;
  questionType?: string;
  scenarioFamily?: string;
}): Promise<ReadingMaterial> {
  await ensureAccessToken();
  const query = `mutation GenerateReading($exam: String!, $topic: String!, $level: String, $sourceIds: [String!], $band: Float, $stage: String, $section: String, $skillFocus: String, $questionType: String, $scenarioFamily: String) {
      generateReading(exam: $exam, topic: $topic, level: $level, sourceIds: $sourceIds, band: $band, stage: $stage, section: $section, skillFocus: $skillFocus, questionType: $questionType, scenarioFamily: $scenarioFamily) {
        id exam language level topic band stage section skillFocus questionType scenarioFamily title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus status generationProgress generationMessage
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey type paragraphRef evidence headings summaryText wordBank answers statements { id text answer } }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ generateReading: ReadingMaterial }>(query, input);
  return data.generateReading;
}

export async function readingMaterials(exam?: string): Promise<ReadingMaterial[]> {
  await ensureAccessToken();
  const query = `query ReadingMaterials($exam: String) {
      readingMaterials(exam: $exam) {
        id exam language level topic band stage section skillFocus questionType scenarioFamily title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus status generationProgress generationMessage
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey type paragraphRef evidence headings summaryText wordBank answers statements { id text answer } }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ readingMaterials: ReadingMaterial[] }>(query, { exam });
  return data.readingMaterials;
}

export async function readingMaterial(id: string): Promise<ReadingMaterial> {
  await ensureAccessToken();
  const query = `query ReadingMaterial($id: ID!) {
      readingMaterial(id: $id) {
        id exam language level topic band stage section skillFocus questionType scenarioFamily title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus status generationProgress generationMessage
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey type paragraphRef evidence headings summaryText wordBank answers statements { id text answer } }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ readingMaterial: ReadingMaterial }>(query, { id });
	return data.readingMaterial;
}

export async function deleteReadingMaterial(materialId: string): Promise<boolean> {
  await ensureAccessToken();
  const data = await request<{ deleteReadingMaterial: boolean }>(
    `mutation DeleteReadingMaterial($materialId: ID!) { deleteReadingMaterial(materialId: $materialId) }`,
    { materialId }
  );
  return data.deleteReadingMaterial;
}

export async function retryReadingAudio(materialId: string): Promise<ReadingMaterial> {
  await ensureAccessToken();
  const query = `mutation RetryReadingAudio($materialId: ID!) {
      retryReadingAudio(materialId: $materialId) {
        id exam language level topic band stage section skillFocus questionType scenarioFamily title passage vocabulary sourceIds generationNote audioUrl audioUrls audioStatus status generationProgress generationMessage
        vocabularyItems { word pos meanings }
        associationSentences
        grammarInsights { sentence difficultyPoints studySuggestions }
        questions { question options answerKey type paragraphRef evidence headings summaryText wordBank answers statements { id text answer } }
      }
    }`;
  const data = await requestWithAnswerKeyFallback<{ retryReadingAudio: ReadingMaterial }>(query, { materialId });
  return data.retryReadingAudio;
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
