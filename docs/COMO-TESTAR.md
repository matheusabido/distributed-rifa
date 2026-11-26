# Como testar tudo

Você precisa do **Docker** (Docker Desktop) e, para os scripts, de **Git Bash**
com **jq** (`winget install jqlang.jq`).

## 1. Subir

Na pasta do projeto:

```bash
cp .env.example .env        # só na primeira vez
docker compose up --build
```

| Endereço | O quê |
|---|---|
| **http://localhost:3000** | o site (front + APIs balanceadas) |
| localhost:8081 / 8082 / 8083 | cada API direto |
| localhost:5431 / 5432 | db1 / db2 direto |

Na primeira vez, o db2 espera o db1 ficar pronto e copia tudo dele. No log
aparece `[db2] cópia pronta`.

> Se você tiver um Postgres instalado no Windows usando a porta 5432, o db2
> não sobe. Troque `"5432:5432"` por `"5433:5432"` no `docker-compose.yml`.

## 2. Usar o site

Abra http://localhost:3000, crie uma conta, crie e abra uma rifa. Depois abra
a mesma rifa em **outra aba** e compre um número: ele fica ocupado na outra
aba na hora.

## 3. Painel de teste

Dê dois cliques em `tools/painel-teste.html`. São 3 colunas, uma por API, e
cada uma mostra:
- quem ela acha que é a **mestre**, e o estado das APIs e dos bancos;
- os eventos de tempo real que chegam nela.

Deixe aberto durante os testes abaixo.

## 4. Roteiro de falhas automático

Em outro Git Bash, com tudo de pé:

```bash
./tools/teste-falhas.sh
```

Ele faz tudo sozinho e mostra o resultado de cada passo:

1. **Para a API 1 (mestre).** A API 2 vira mestre e as compras pelo site
   continuam dando `201`.
2. **Para o db1 (primário).** Por ~3s as compras dão `503`, depois a mestre
   promove o db2 e voltam a dar `201`.
3. **Religa o db1.** Ele volta como **réplica** do db2.
4. **Religa a API 1.** Ela volta a ser a mestre.
5. Confere se os dois bancos têm os mesmos números vendidos.

Resultado quando rodei aqui:

```
1) derrubando a API 1 (mestre)
{"eu":2,"mestre":2,"apis":["api1=down","api2=up","api3=up"], ...}
2) derrubando o db1 (banco primário)
00:57:08 compra 10: 503
00:57:09 compra 11: 503
00:57:10 compra 12: 201   ← db2 promovido
3) religando o db1
[db1] eu era primário, mas db2 virou primário enquanto eu estava fora
[db1] copiando todos os dados de db2 (pg_basebackup) e virando RÉPLICA
4) religando a API 1
{"eu":3,"mestre":1, ...}
db1: 11 números vendidos
db2: 11 números vendidos
```

## 5. Testar na mão

```bash
docker compose stop api1      # derruba
docker compose start api1     # religa
docker compose logs -f api2   # acompanha o que a API 2 está vendo
curl http://localhost:8082/cluster    # o que a API 2 acha do sistema
```

Os logs do monitor são fáceis de ler:

```
[api 3] [monitor] DESCOBRI: api1 up -> down. Avisando as outras APIs.
[api 2] [monitor] a api3 me avisou: api1 up -> down
[api 2] [monitor] a API mestre agora é a api2
[api 2] [monitor] SOU A MESTRE e não há banco primário: promovendo db2
```

## 6. Olhar os bancos

```bash
docker compose exec db2 psql -U postgres -d rifa -c "select * from tickets"
docker compose exec db1 psql -U postgres -tAc "select pg_is_in_recovery()"   # t = réplica
```

## Desligar

```bash
docker compose down      # para tudo (os dados ficam guardados)
docker compose down -v   # para e APAGA os dados: volta ao db1 primário, db2 réplica
```

Depois de testar falhas, o db2 pode ter ficado como primário. Isso é
normal, e o sistema funciona igual. Para voltar ao começo, use `down -v`.
