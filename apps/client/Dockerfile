FROM node:20-alpine AS build
WORKDIR /app
ARG VITE_API_URL=/graphql
ARG VITE_SENTRY_DSN=""
ENV VITE_API_URL=${VITE_API_URL}
ENV VITE_SENTRY_DSN=${VITE_SENTRY_DSN}
COPY package.json package-lock.json ./
COPY apps/client/package.json apps/client/package.json
RUN npm ci --workspace apps/client
COPY apps/client apps/client
RUN npm run build --workspace apps/client

FROM nginx:1.27-alpine
COPY --from=build /app/apps/client/dist /usr/share/nginx/html
COPY apps/client/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
