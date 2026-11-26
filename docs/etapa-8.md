# Etapa 8: o front do seu colega + nginx

## Como ficou

```
navegador ──► http://localhost:3000  (nginx, container "web")
                 │
                 ├── /           → front React (já compilado)
                 ├── /api/...    → api1, api2 ou api3 (balanceado)
                 └── /api/ws     → websocket, também balanceado
```

- O front entrou em `frontend/` sem mudança de tela. Só ajustei o
  `src/api.js` para aceitar `VITE_API_URL=/api` (caminho relativo): o
  websocket passa a usar o mesmo endereço da página.
- `docker/web/Dockerfile` compila o front com Node (`npm ci && npm run
  build`) e entrega os arquivos prontos ao nginx.
- `docker/web/nginx.conf`:
  - **balanceia** entre as 3 APIs;
  - se uma API não responde, **tenta a próxima** (`proxy_next_upstream`);
  - um `503` da API ("banco indisponível") **não** conta como API caída,
    e vai direto para o navegador.
- Porta **3000** e não 8080, por causa do Apache que você tem rodando.

## Testado

- Fluxo completo pelo site: cadastro, criar, abrir, comprar, sortear.
- Com a página da rifa aberta, comprei um número direto pela API 3. O botão
  ficou "vendido" sozinho, sem recarregar (websocket + gRPC entre APIs).
- Com a API 1 parada, 12 compras seguidas pelo site deram 201 (o nginx
  desviou para as outras).

## Rodar o front sem Docker (para o seu colega desenvolver)

```bash
cd frontend
npm install
VITE_API_URL=http://localhost:8081 npm run dev
```
