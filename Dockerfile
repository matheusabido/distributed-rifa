# Etapa de build: compila o binário com a imagem oficial do Go.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /apinode ./cmd/apinode

# Imagem final: só o binário, bem pequena.
FROM alpine:3.20
COPY --from=build /apinode /apinode
EXPOSE 8080 9000
ENTRYPOINT ["/apinode"]
