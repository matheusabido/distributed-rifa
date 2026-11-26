# Etapa 2: rotas, login e regras da rifa

## O que foi feito

```
internal/
├── store/store.go         ← ÚNICO lugar com SQL (pgxpool)
├── auth/auth.go           ← senha (bcrypt) + token (JWT) + middleware Bearer
└── httpapi/
    ├── server.go          ← lista de rotas
    ├── handlers.go        ← o que cada rota faz
    ├── errors.go          ← erro de regra -> status HTTP
    ├── events.go          ← "avisar que algo mudou" (só loga por enquanto)
    └── api_test.go        ← testes de ponta a ponta com Postgres de verdade
cmd/apinode/main.go        ← liga tudo e sobe o servidor
```

## As camadas (quem chama quem)

```
requisição HTTP
   │
   ▼
auth.Middleware   (só nas rotas protegidas: confere o token Bearer)
   │
   ▼
handler           (lê JSON, valida, chama o store, avisa, responde)
   │
   ▼
store             (SQL no banco do seu amigo via pgxpool)
```

Cada camada só conhece a de baixo. O handler nunca escreve SQL, e o store
nunca sabe que existe HTTP. Se o seu amigo renomear uma coluna, só o
`store.go` muda.

## Rotas

| Método | Rota | Precisa de token? | O que faz |
|---|---|---|---|
| POST | `/auth/registrar` | não | cria conta, devolve token |
| POST | `/auth/login` | não | devolve token |
| GET | `/rifas` | não | rifas abertas (`?status=closed` / `drawn` para outras) |
| GET | `/rifas/:id` | não | rifa + números vendidos (só nome) |
| POST | `/rifas/:id/numeros/:numero/comprar` | **não** | compra um número |
| GET | `/minhas-rifas` | sim | minhas rifas, inclusive rascunhos |
| POST | `/rifas` | sim | cria rifa (nasce como rascunho) |
| PUT | `/rifas/:id` | sim | edita |
| POST | `/rifas/:id/abrir` | sim | rascunho/fechada → aberta |
| POST | `/rifas/:id/fechar` | sim | aberta → fechada |
| POST | `/rifas/:id/sortear` | sim | fechada → sorteada |
| GET | `/rifas/:id/compradores` | sim | lista com contato (só o dono) |

Exemplos prontos para copiar estão em [exemplos.sh](exemplos.sh).

### Ciclo de vida da rifa

```
draft ──abrir──► open ──fechar──► closed ──sortear──► drawn
                  ▲                  │
                  └──────abrir───────┘   (pode reabrir antes de sortear)
```

- Só compra quando está `open`.
- Só sorteia quando está `closed`, e sorteia entre os números **vendidos**.
- Só o dono abre, fecha, edita e sorteia (se não for o dono, responde `403`).

## Login com Bearer, passo a passo

1. `POST /auth/registrar {"email","senha"}`: a senha vira hash **bcrypt**
   antes de ir para o banco. Nem quem tem acesso ao banco vê a senha.
2. A resposta traz um **token JWT**. Ele contém o ID do usuário e uma
   assinatura feita com `JWT_SECRET`, e vale 24h.
3. Nas rotas protegidas, o cliente manda `Authorization: Bearer <token>`.
4. O `auth.Middleware` confere a assinatura. Se alguém alterar o token, a
   assinatura não bate e a resposta é `401`.

**Por que isso importa no sistema distribuído:** qualquer réplica da API
valida o token sozinha, porque todas conhecem o mesmo `JWT_SECRET`. Você faz
login na API 1 e usa o token na API 2 sem problema.

## O "lock durante a escrita" (em `store.BuyNumber`)

Quando 50 pessoas clicam no número 42 ao mesmo tempo:

1. Cada compra abre uma **transação**.
2. `SELECT status ... FROM raffles ... FOR SHARE` lê a rifa com um lock
   *compartilhado*. Várias compras podem segurar esse lock juntas, mas o dono
   não consegue **fechar** a rifa no meio delas (fechar pede lock exclusivo e
   espera as compras terminarem).
3. `INSERT INTO tickets (raffle_id, number, ...)`: como `(raffle_id, number)`
   é chave primária, o Postgres deixa **só um** INSERT passar. Os outros 49
   recebem o erro `23505`.
4. A API transforma `23505` em `409 Conflict: número já vendido`.

O teste `TestCompraConcorrente` faz exatamente isso, e o resultado é sempre
1 sucesso e 49 conflitos.

### E as operações do dono? (`ownerTx`)

Abrir, fechar, editar e sortear seguem um molde só:
`BEGIN` → `SELECT ... FOR UPDATE` (lock exclusivo na rifa) → confere se é o
dono → faz a mudança → `COMMIT`. Se qualquer passo falhar, é `ROLLBACK` e
nada fica gravado pela metade.

## O sorteio

O número é sorteado **na API** com `crypto/rand` (aleatório de verdade, não
previsível), e o resultado é gravado no banco. Assim o mestre e as réplicas
do seu amigo guardam o mesmo vencedor. Se cada réplica sorteasse sozinha,
cada uma teria um vencedor diferente.

## Privacidade do comprador

A compra pede nome e contato. O público (`GET /rifas/:id` e os eventos) vê
**só o nome**. O contato aparece só para o dono, em `/rifas/:id/compradores` e
na resposta do sorteio, para ele conseguir entregar o prêmio.

## Erros

Todo erro sai no formato `{"erro": "mensagem"}`, com o status certo:

| Situação | Status |
|---|---|
| JSON ou campo inválido | 400 |
| sem token / token inválido / senha errada | 401 |
| não é o dono | 403 |
| rifa não existe | 404 |
| número já vendido, rifa no status errado, e-mail repetido | 409 |
| qualquer outra coisa (ex: banco fora do ar) | 500 (detalhe só no log) |

## Eventos (gancho para a etapa 3)

Toda escrita bem-sucedida chama `s.notify.Notify(tipo, rifaID, dados)`. Por
enquanto isso só escreve no log (`[evento] ticket_bought rifa=1`). Na etapa
3, o mesmo `Notify` vai mandar o evento por websocket para os navegadores e
por gRPC para as outras APIs, sem mexer em nenhum handler.

## Como rodar

```bash
# precisa de um Postgres com o db/schema.sql aplicado
psql postgres://rifa:rifa@localhost:5432/rifa -f db/schema.sql

make build
NODE_ID=1 DATABASE_URL=postgres://rifa:rifa@localhost:5432/rifa ./bin/apinode

# testes (APAGAM as tabelas do banco indicado: use um banco de teste)
TEST_DATABASE_URL=postgres://rifa:rifa@localhost:5432/rifa go test ./...
```

Resultado dos testes nesta etapa: `TestFluxoCompleto` e
`TestCompraConcorrente` passando.

## Para combinar com seu amigo

Nada novo além do que estava na etapa 1: o `db/schema.sql` e a string de
conexão.

## Próxima etapa

Etapa 3: websocket (`GET /ws`) e o broadcast gRPC entre as APIs, para todo
mundo ver o número ficar ocupado na hora.
