# GraphQL API（Product）

Endpoint: `POST /graphql`

## 认证

- Header: `Authorization: Bearer <access_token>`

## Mutations

- `register(email, password) -> { accessToken }`
- `login(email, password) -> { accessToken }`
- `refresh(accessToken) -> { accessToken }`
- `logout() -> Boolean`
- `updateProfile(nickname, avatarUrl, bio) -> User`
- `generateTheater(input) -> Theater`
- `submitAnswers(theaterId, answers) -> PracticeResult`
- `toggleFavorite(theaterId, favorite) -> Boolean`
- `shareTheater(theaterId) -> String`
- `startRoleplay(theaterId, userRole) -> RoleplaySession`
- `submitRoleplayReply(sessionId, text) -> RoleplaySession`
- `endRoleplay(sessionId) -> RoleplaySession`

## Queries

- `me -> User`
- `theater(id) -> Theater`
- `myTheaters(language, status, favorite) -> [Theater]`
- `courses(language) -> [Course]`
- `roleplaySession(sessionId) -> RoleplaySession`

## 健康检查

- `GET /healthz`
- `GET /readyz`（可选：按部署需求配置）
