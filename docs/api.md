# GraphQL API（Product）

Endpoint: `POST /graphql`

## 认证

- Header: `Authorization: Bearer <access_token>`

## Mutations

- `register(email, password) -> { accessToken }`
- `login(email, password) -> { accessToken }`
- `refresh(refreshToken) -> { accessToken, refreshToken }`（refresh token 会轮换；`accessToken` 参数仅用于一次发布周期的迁移兼容）
- `logout() -> Boolean`
- `updateProfile(nickname, avatarUrl, bio) -> User`
- `updateModelConfig(model, baseURL, apiKey, clearApiKey) -> ModelConfig`
- `updateTTSConfig(provider, model, baseURL, voice, apiKey, clearApiKey) -> TTSConfig`
- `generateTheater(input) -> Theater`
- `submitAnswers(theaterId, answers) -> PracticeResult`
- `toggleFavorite(theaterId, favorite) -> Boolean`
- `shareTheater(theaterId) -> String`
- `startRoleplay(theaterId, userRole) -> RoleplaySession`
- `submitRoleplayReply(sessionId, text) -> RoleplaySession`
- `endRoleplay(sessionId) -> RoleplaySession`

## Queries

- `me -> User`
- `modelConfig -> ModelConfig`
- `ttsConfig -> TTSConfig`
- `theater(id) -> Theater`
- `myTheaters(language, status, favorite) -> [Theater]`
- `courses(language) -> [Course]`
- `roleplaySession(sessionId) -> RoleplaySession`

## 健康检查

- `GET /healthz`
- `GET /readyz`（可选：按部署需求配置）
