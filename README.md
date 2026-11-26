# Rifa distribuída

Rifa beneficente de 200 números (prêmio: caixa de bombom; arrecadação para um
gatinho paraplégico), construída como um sistema distribuído.

## Visão geral

```
                     navegador
                         │
                web (nginx :3000)  ← front React + balanceamento
          ┌──────────────┼──────────────┐
        api1           api2           api3     ← Go + Fiber
          └──── gRPC: Broadcast + Monitor ─────┘
                         │ pgx: "quem for o primário"
                ┌────────┴────────┐
               db1  ──réplica──►  db2          ← Postgres 19
```

- **3 APIs** com hierarquia (1 > 2 > 3). A mestre é a mais alta que está viva.
- Cada API **checa as outras e os bancos** num segundo diferente
  (1, 7, 13... / 3, 9, 15... / 5, 11, 17...). Quem descobre uma queda avisa
  todas por **gRPC**.
- **2 bancos:** db1 primário, db2 réplica em tempo real. Se o db1 cai, a API
  mestre promove o db2. O db1 volta como réplica.
- **Tempo real:** cada escrita vira evento via websocket, e o gRPC repassa
  para os navegadores conectados nas outras APIs.
- Datas sempre em **UTC**.

## Como rodar

```bash
cp .env.example .env
docker compose up --build
```

Abra **http://localhost:3000**. O passo a passo de testes, incluindo derrubar
API e banco, está em [docs/COMO-TESTAR.md](docs/COMO-TESTAR.md).

## Pastas

| Pasta | O quê | Quem |
|---|---|---|
| `cmd/`, `internal/`, `proto/`, `gen/` | API em Go | Rafael |
| `docker/db/` | imagem dos bancos | amigo do Rafael (+ réplica automática) |
| `frontend/` | front React + Vite | colega do Rafael |
| `docker/web/` | nginx + build do front | |
| `db/schema.sql` | tabelas | |
| `tools/` | painel de teste e roteiro de falhas | |

## Etapas (explicação de cada parte)

| Etapa | O quê | Explicação |
|---|---|---|
| 1 | Estrutura, contrato gRPC e esquema do banco | [docs/etapa-1.md](docs/etapa-1.md) |
| 2 | Rotas, auth Bearer, regras da rifa (pgxpool) | [docs/etapa-2.md](docs/etapa-2.md) |
| 3 | Websocket + broadcast gRPC entre APIs | [docs/etapa-3.md](docs/etapa-3.md) |
| 4 | CORS + guia da API para o frontend | [docs/GUIA-FRONTEND.md](docs/GUIA-FRONTEND.md) |
| 5 | docker-compose e variáveis | [docs/variaveis.md](docs/variaveis.md) |
| 6 | Dois bancos (primário + réplica) e UTC | [docs/etapa-6.md](docs/etapa-6.md) |
| 7 | 3 APIs, hierarquia, checagem e avisos | [docs/etapa-7.md](docs/etapa-7.md) |
| 8 | Front + nginx | [docs/etapa-8.md](docs/etapa-8.md) |

## Comandos de desenvolvimento

```bash
make proto   # gera gen/rifapb a partir de proto/rifa.proto
make build   # compila bin/apinode
TEST_DATABASE_URL=postgres://... go test ./...   # testes (APAGAM as tabelas!)
```
