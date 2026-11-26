# Etapa 1: estrutura, contrato gRPC e esquema do banco

## O que foi feito

```
rifa/
├── proto/rifa.proto        ← contrato gRPC entre as APIs (Broadcast)
├── gen/rifapb/             ← código Go GERADO pelo protoc (não editar à mão)
├── db/schema.sql           ← PROPOSTA de tabelas para combinar com o banco
├── internal/config/        ← leitura de variáveis de ambiente
├── cmd/apinode/main.go     ← programa da API (esqueleto por enquanto)
├── Makefile                ← make proto / make build
└── go.mod
```

Ainda não há rotas. O objetivo desta etapa é deixar claro **com quem a API
conversa e como**.

## A API conversa com dois tipos de coisa

### 1. Com o banco: SQL via `pgxpool`

`pgxpool` é um *pool* de conexões: em vez de abrir uma conexão por requisição
(lento), a API mantém algumas abertas e reaproveita.

**Como a API acha o mestre?** O banco tem mestre + réplicas, e só o mestre
aceita escrita. O pgx resolve isso sozinho se a string de conexão listar todos
os nós:

```
postgres://rifa:senha@db1:5432,db2:5432,db3:5432/rifa?target_session_attrs=read-write
```

`target_session_attrs=read-write` diz ao pgx: "tente os hosts em ordem e fique
com o que aceita escrita". Se o mestre cair e outro for eleito, as novas
conexões do pool vão parar no novo mestre. **Isso depende de como seu amigo
implementar o banco**, então vale confirmar com ele.

### 2. Com as outras APIs: gRPC `Broadcast`

Cada navegador fica conectado por websocket em **uma** API. Se a compra passou
pela API 1, quem está na API 2 também precisa saber. A API 1 chama
`Publish(evento)` na API 2, e a API 2 repassa aos navegadores dela.

### gRPC em 1 minuto (para explicar para alguém)

Pense no gRPC como "chamar uma função que está em outro computador".

1. No `.proto` você escreve **o formato da mensagem** (`Event`: tipo, id da
   rifa, JSON) e **a função** (`Publish(Event)`). É como declarar a assinatura
   de uma função.
2. O comando `make proto` lê esse arquivo e **escreve o código Go por você**
   (pasta `gen/`). Ninguém mexe nessa pasta.
3. Na API que avisa: `cliente.Publish(ctx, &rifapb.Event{...})`. Parece uma
   chamada de função normal, mas ela viaja pela rede até a outra API.
4. Na API que recebe: escrevemos a função `Publish`, que pega o evento e manda
   para os navegadores conectados nela.

Comparando com HTTP: seria como um `POST /evento` com JSON, só que o formato
é combinado antes no `.proto`, então os dois lados nunca discordam dos campos.

O `protoc` lê `proto/rifa.proto` e gera em Go:
- a struct `rifapb.Event`;
- um **cliente** (`rifapb.NewBroadcastClient(conn)`), para chamar a outra API
  como se fosse uma função local;
- uma **interface de servidor** (`rifapb.BroadcastServer`), que vamos
  implementar na etapa 3.

## O esquema do banco (`db/schema.sql`)

É uma **proposta** para você mostrar ao seu amigo. Três tabelas:

- `users`: só quem cria rifa tem conta. Senha guardada com bcrypt.
- `raffles`: a rifa, com `status` = `draft` → `open` → `closed` → `drawn`.
- `tickets`: cada número vendido.

### O "lock durante a escrita"

No diagrama, o banco fica travado durante a escrita para ninguém comprar o
mesmo número junto. No Postgres, isso sai de graça com a chave primária
`(raffle_id, number)` em `tickets`:

1. Ana e Beto clicam no 42 ao mesmo tempo.
2. As duas APIs fazem `INSERT INTO tickets (raffle_id, number, ...) VALUES (1, 42, ...)`.
3. O banco deixa só um entrar. O outro recebe erro de unicidade (código `23505`).
4. A API transforma esse erro em `409 Conflict: número já vendido`.

Para "comprar só se a rifa está aberta", a API usa uma transação com
`SELECT ... FOR UPDATE` na rifa. Isso trava a linha da rifa até o fim da
transação, então ninguém fecha a rifa no meio da compra.

## Decisões tomadas

- **Auth com JWT**: o token é assinado com `JWT_SECRET`, o mesmo em todas as
  APIs. Qualquer réplica valida o token sem consultar o banco.
- **Preço em centavos** (`BIGINT`), nunca `float`, para não ter erro de
  arredondamento com dinheiro.
- **Compra marca como comprado direto**, sem etapa de pagamento.
- **API sem estado**: nada fica guardado na memória da API além das conexões
  websocket. Por isso dá para ter quantas réplicas quiser.

## Como testar o que existe

```bash
make proto && make build
NODE_ID=1 PEER_APIS=api2:9000 ./bin/apinode
# [api 1] http=:8080 grpc=:9000 outras_apis=[api2:9000]
```

## Para combinar com seu amigo

1. O esquema em `db/schema.sql` serve para ele? Quem cria as tabelas?
2. A string de conexão com vários hosts + `target_session_attrs=read-write`
   funciona com o banco dele? Se não, como a API descobre o mestre?

## Próxima etapa

Etapa 2: rotas HTTP com Fiber, cadastro/login com JWT, e as regras da rifa
(criar, editar, abrir, fechar, comprar, sortear) usando pgxpool.
