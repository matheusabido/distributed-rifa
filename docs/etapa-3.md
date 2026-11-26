# Etapa 3: websocket e broadcast gRPC entre as APIs

## O problema

Cada navegador fica conectado por websocket em **uma** réplica da API (quem
escolhe é o balanceador). Se a Ana compra o 42 pela API 1, o Beto, que está
na API 2, também precisa ver o 42 ficar ocupado na hora.

## A solução

```
 navegador A ──ws──► API 1                      API 2 ◄──ws── navegador B
                       │                          ▲
   POST .../42/comprar │                          │
                       ▼                          │
                  grava no banco                  │
                       │                          │
                Notify("ticket_bought")           │
                   │            │                 │
         hub.Broadcast     gRPC Publish ──────────┘
          (navegador A)                    hub.Broadcast (navegador B)
```

1. A rota de compra grava no banco (etapa 2) e chama `Notify`.
2. `Notify` manda o evento para os navegadores **desta** API (o Hub).
3. `Notify` também chama `Publish` via **gRPC** em cada uma das **outras** APIs.
4. Cada API que recebe `Publish` repassa para os **seus** navegadores.
5. Quem recebe `Publish` **não** reenvia para ninguém, senão o evento ficaria
   rodando em círculo entre as APIs.

## Arquivos novos

```
internal/realtime/
├── hub.go       ← os navegadores conectados nesta API
└── cluster.go   ← Notify (manda) + Publish (recebe via gRPC)
internal/httpapi/realtime_test.go  ← teste com 2 APIs reais
```

Mudanças: `server.go` ganhou a rota `GET /ws`, e `cmd/apinode/main.go` agora
sobe **duas portas** (HTTP `:8080` para navegadores e gRPC `:9000` para as
outras APIs).

## `hub.go`: os navegadores conectados

- Cada navegador vira um `client` com uma **fila** (canal Go com espaço para
  32 mensagens).
- `Broadcast(msg)` só coloca a mensagem na fila de cada um, o que é
  instantâneo.
- Cada navegador tem uma goroutine própria que tira da fila e escreve no
  websocket. Assim um navegador com internet ruim não atrasa os outros.
- Se a fila de alguém encher (aba travada), ele é desconectado. Quando o
  navegador reconectar, ele busca o estado atual pela API HTTP.

## `cluster.go`: o gRPC

O `Cluster` faz dois papéis ao mesmo tempo:

| Papel | Método | Quem chama |
|---|---|---|
| **cliente** gRPC | `Notify` → `p.client.Publish(...)` | as rotas, depois de gravar |
| **servidor** gRPC | `Publish(ctx, ev)` | as outras APIs |

Lembrando do "gRPC em 1 minuto" da etapa 1: `p.client.Publish(ctx, ev)`
parece uma chamada de função normal, mas ela viaja pela rede e executa o
método `Publish` do `Cluster` **da outra API**.

Detalhes:

- **`grpc.NewClient` não conecta na hora.** Ele conecta na primeira chamada e
  reconecta sozinho. Por isso não importa qual API sobe primeiro.
- **Em segundo plano** (`go c.publishToPeers(ev)`): quem comprou recebe a
  resposta HTTP sem esperar o aviso chegar às outras APIs.
- **Timeout de 2s por API.** Se uma API está fora do ar, só aparece um log de
  erro. Ela não tem navegadores conectados mesmo, e quando voltar os
  navegadores reconectam e recarregam o estado.

## O que chega no navegador

```json
{
  "tipo": "ticket_bought",
  "rifa_id": 1,
  "dados": { "rifa_id": 1, "numero": 42, "nome": "Ana" },
  "origem": 1
}
```

`origem` mostra qual API gerou o evento, o que é ótimo para demonstrar o
sistema distribuído funcionando.

| tipo | quando | dados |
|---|---|---|
| `ticket_bought` | alguém comprou | número e nome (sem contato) |
| `raffle_status` | abriu ou fechou | a rifa inteira |
| `raffle_updated` | dono editou (rifa não-rascunho) | a rifa inteira |
| `raffle_drawn` | sorteio | número e nome do vencedor |

Rascunhos não geram evento, porque só o dono pode vê-los.

## Como testar à mão (duas APIs no seu PC)

```bash
make build
DB=postgres://rifa:rifa@localhost:5432/rifa

# terminal 1
NODE_ID=1 HTTP_ADDR=:8081 GRPC_ADDR=:9001 PEER_APIS=localhost:9002 DATABASE_URL=$DB ./bin/apinode
# terminal 2
NODE_ID=2 HTTP_ADDR=:8082 GRPC_ADDR=:9002 PEER_APIS=localhost:9001 DATABASE_URL=$DB ./bin/apinode
```

Abra o console do navegador (F12) e conecte na **API 2**:

```js
const ws = new WebSocket("ws://localhost:8082/ws");
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

Agora compre pela **API 1** (rode `API=http://localhost:8081 ./docs/exemplos.sh`).
O console, conectado na API 2, mostra os eventos com `"origem": 1`. No log da
API 2 aparece `[grpc] evento ticket_bought da api 1, repassando para 1 navegador(es)`.

## Testes

```
TestFluxoCompleto        PASS   (etapa 2)
TestCompraConcorrente    PASS   (etapa 2)
TestTempoRealEntreAPIs   PASS   (novo)
```

O teste novo sobe duas APIs com portas HTTP e gRPC de verdade, conecta um
"navegador" em cada uma, compra pela API 1 e confere se **os dois** receberam
o evento, sem o contato do comprador. Também roda limpo com `go test -race`,
que procura acesso concorrente errado à memória.

## Próxima etapa

Etapa 4: o frontend em HTML/CSS/JS, com a grade de 200 números que atualiza
sozinha usando esses eventos.
