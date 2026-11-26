# 1) Compila o frontend React com Node.
FROM node:22-alpine AS build
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
# A API fica no mesmo endereço, em /api (o nginx repassa).
ENV VITE_API_URL=/api
RUN npm run build

# 2) Serve os arquivos prontos com nginx.
FROM nginx:1.27-alpine
COPY docker/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
