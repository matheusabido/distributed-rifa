# Guia da API para quem faz o frontend

Tudo o que o frontend precisa saber para conversar com a API. Todas as
respostas são JSON.

## Endereço

| Ambiente | HTTP | Websocket |
|---|---|---|
| docker compose (API 1) | `http://localhost:8081` | `ws://localhost:8081/ws` |
| docker compose (API 2) | `http://localhost:8082` | `ws://localhost:8082/ws` |

Qualquer uma das duas serve, porque elas são iguais. O CORS já está liberado,
então o front pode rodar em outro endereço (Live Server, Vite etc.).

```js
const API = "http://localhost:8081";
```

## Erros

Todo erro vem como `{"erro": "mensagem em português"}`. A mensagem já pode ser
mostrada ao usuário.

| Status | Quando |
|---|---|
| 400 | campo faltando ou inválido |
| 401 | sem token, token expirado ou senha errada → mandar para o login |
| 403 | não é o dono da rifa |
| 404 | rifa não existe |
| 409 | número já vendido, rifa no status errado, e-mail repetido |

```js
async function api(method, path, body, token) {
  const res = await fetch(API + path, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token && { Authorization: "Bearer " + token }),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.erro);
  return data;
}
```

## Público (sem login)

### Listar rifas abertas
`GET /rifas`. Também aceita `?status=closed` e `?status=drawn`.

```json
[
  {
    "id": 1, "dono_id": 1,
    "titulo": "Rifa do gatinho", "descricao": "Ajude no tratamento",
    "premio": "Caixa de bombom",
    "preco_centavos": 500,
    "total_numeros": 200,
    "status": "open",
    "numero_sorteado": null,
    "vendidos": 37,
    "criada_em": "2026-11-25T03:42:28Z"
  }
]
```

- **O preço vem em centavos.** Para mostrar:
  `(r.preco_centavos / 100).toLocaleString("pt-BR", {style: "currency", currency: "BRL"})` → `R$ 5,00`.
- `status`: `open` (aberta), `closed` (fechada, aguardando sorteio) ou
  `drawn` (sorteada).

### Detalhe de uma rifa (para montar a grade)
`GET /rifas/:id`

```json
{
  "rifa": { ...mesmo formato acima... },
  "vendidos": [
    { "rifa_id": 1, "numero": 7,  "nome": "Ana",  "comprado_em": "..." },
    { "rifa_id": 1, "numero": 42, "nome": "Beto", "comprado_em": "..." }
  ]
}
```

Os números vão de `1` até `total_numeros`. Os que **não** estão em
`vendidos` estão livres.

### Comprar um número (sem conta)
`POST /rifas/:id/numeros/:numero/comprar`

```json
{ "nome": "Ana", "contato": "65 99999-0000" }
```

- `201`: comprado.
- `409 "número já vendido"`: alguém comprou antes. Marque como ocupado.
- `409` com outra mensagem: a rifa não está aberta.

## Dono da rifa (precisa de login)

### Cadastro e login
`POST /auth/registrar` ou `POST /auth/login`, com o corpo:

```json
{ "email": "rafael@exemplo.com", "senha": "123456" }
```

Os dois respondem `{ "token": "eyJ...", "usuario_id": 1 }`. Guarde o token
(ex: `localStorage`) e mande em toda rota abaixo:

```
Authorization: Bearer eyJ...
```

O token vale 24h. Quando vier `401`, mande o usuário fazer login de novo.

### Rotas

| Método | Rota | Corpo | Resposta |
|---|---|---|---|
| GET | `/minhas-rifas` | — | lista das rifas do usuário, inclusive rascunhos |
| POST | `/rifas` | ver abaixo | a rifa criada (status `draft`) |
| PUT | `/rifas/:id` | ver abaixo | a rifa editada |
| POST | `/rifas/:id/abrir` | — | a rifa (`open`) |
| POST | `/rifas/:id/fechar` | — | a rifa (`closed`) |
| POST | `/rifas/:id/sortear` | — | `{ "rifa": {...}, "vencedor": { "numero", "nome", "contato" } }` |
| GET | `/rifas/:id/compradores` | — | lista de vendidos **com contato** |

Corpo de criar/editar:

```json
{
  "titulo": "Rifa do gatinho",
  "descricao": "Ajude no tratamento",
  "premio": "Caixa de bombom",
  "preco_centavos": 500,
  "total_numeros": 200
}
```

`titulo` e `premio` são obrigatórios. `total_numeros` é opcional (o padrão é
200) e só muda enquanto a rifa é rascunho.

### Ciclo de vida (que botões mostrar)

```
draft ──abrir──► open ──fechar──► closed ──sortear──► drawn
                  ▲                  │
                  └──────abrir───────┘
```

| status | botões |
|---|---|
| `draft` | Editar, Abrir |
| `open` | Editar, Fechar |
| `closed` | Editar, Abrir (reabrir), Sortear |
| `drawn` | nenhum (mostrar vencedor) |

## Tempo real (websocket)

Conecte uma vez ao abrir a página. O servidor manda um JSON a cada mudança,
vinda de **qualquer** réplica da API:

```js
function conectar() {
  const ws = new WebSocket(API.replace("http", "ws") + "/ws");
  ws.onmessage = (e) => {
    const ev = JSON.parse(e.data);
    if (ev.rifa_id !== rifaAtual) return; // ignore outras rifas
    switch (ev.tipo) {
      case "ticket_bought":  marcarVendido(ev.dados.numero, ev.dados.nome); break;
      case "raffle_status":  atualizarRifa(ev.dados); break;   // abriu/fechou
      case "raffle_updated": atualizarRifa(ev.dados); break;   // dono editou
      case "raffle_drawn":   mostrarVencedor(ev.dados.numero, ev.dados.nome); break;
    }
  };
  ws.onclose = () => setTimeout(() => { recarregarRifa(); conectar(); }, 2000);
}
```

Formato de cada mensagem:

```json
{ "tipo": "ticket_bought", "rifa_id": 1, "dados": { "rifa_id": 1, "numero": 42, "nome": "Ana" }, "origem": 2 }
```

| tipo | `dados` |
|---|---|
| `ticket_bought` | `{ rifa_id, numero, nome }` |
| `raffle_status` | a rifa inteira (formato do `GET /rifas`) |
| `raffle_updated` | a rifa inteira |
| `raffle_drawn` | `{ rifa_id, numero, nome }` do vencedor |

`origem` diz qual réplica da API gerou o evento. O front não precisa dele, mas
é legal mostrar numa apresentação.

**Importante:** eventos que acontecem enquanto o websocket está
desconectado se perdem. Por isso, ao reconectar, chame `GET /rifas/:id` de
novo (é o `recarregarRifa()` do exemplo).

## Fluxo sugerido da página da rifa

1. `GET /rifas/:id` → desenha a grade (vendidos em cinza, livres clicáveis).
2. Abre o websocket → cada `ticket_bought` pinta o número de cinza na hora.
3. Clique num número livre → pede nome e contato → `POST .../comprar`.
4. Se der `409`, avisa "alguém comprou antes" e pinta de cinza.
