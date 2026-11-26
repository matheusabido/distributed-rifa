# Variáveis de ambiente

A API lê tudo de variáveis de ambiente (`internal/config/config.go`). Quem
define os valores é quem **inicia** o programa, aqui o docker compose.

## Onde cada uma é definida

```
.env                     ← segredos e o que é IGUAL em todas as APIs
  POSTGRES_PASSWORD, DB_NAME, JWT_SECRET, CORS_ORIGINS

docker-compose.yml
  env_file: .env         ← joga o .env dentro de cada API
  x-api-env              ← o que é igual em todas as APIs (DB_HOSTS, DB_USER...)
  environment:           ← o que é DIFERENTE em cada API (NODE_ID, PEER_APIS)
  ${POSTGRES_PASSWORD}   ← o compose substitui pelo valor do .env
```

O `.env` fica fora do git (`.gitignore`). No git vai o `.env.example`.

## API

| Variável | Obrigatória? | Padrão | Para que serve |
|---|---|---|---|
| `NODE_ID` | **sim** | — | ID desta API (define a hierarquia e os segundos de checagem) |
| `PEER_APIS` | não | vazio | as **outras** APIs, `ID@endereço`: `2@api2:9000,3@api3:9000` |
| `HTTP_ADDR` | não | `:8080` | porta HTTP + websocket |
| `GRPC_ADDR` | não | `:9000` | porta gRPC (Broadcast + Monitor) |
| `DB_HOSTS` | não | `localhost:5432` | os bancos, em ordem: `db1:5432,db2:5432` |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | não | `postgres` / `postgres` / `rifa` | acesso aos bancos |
| `DATABASE_URL` | não | — | se vier, substitui a conexão montada com `DB_*` (para rodar fora do Docker) |
| `JWT_SECRET` | não* | `dev-secret` | assina os tokens. *Troque sempre: `openssl rand -hex 32` |
| `CORS_ORIGINS` | não | `*` | sites que podem chamar a API pelo navegador |

## Bancos (`docker/db/rifa-entrypoint.sh`)

| Variável | Para que serve |
|---|---|
| `POSTGRES_PASSWORD` | senha do `postgres` e do `replicator` |
| `POSTGRES_DB` | nome do banco (`rifa`) |
| `NODE_NAME` | `db1` / `db2` (para os logs) |
| `PEER_HOST` | o outro banco |
| `INITIAL_ROLE` | `primary` ou `standby`: o papel na **primeira** vez |

## Conferir

```bash
docker compose config                   # o compose com o .env já aplicado
docker compose exec api1 env | sort     # variáveis dentro da api1
```
