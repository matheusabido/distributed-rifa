# Etapa 7: 3 APIs com hierarquia, checagem e avisos por gRPC

## As regras

1. **Hierarquia:** API 1 > API 2 > API 3. A **mestre** é a de maior
   hierarquia (menor ID) que está viva. Se a 1 cai, a 2 assume; quando a 1
   volta, ela retoma.
2. **Cada API checa num horário diferente:**

   | API | segundos |
   |---|---|
   | 1 | 1, 7, 13, 19, 25... |
   | 2 | 3, 9, 15, 21, 27... |
   | 3 | 5, 11, 17, 23, 29... |

   Ou seja, alguém olha o sistema **a cada 2 segundos**.
3. Na sua vez, a API checa **as outras APIs** (gRPC `Ping`) e **os dois
   bancos** (conecta e pergunta `pg_is_in_recovery()`).
4. **Quem descobrir primeiro** que algo mudou (caiu ou voltou) avisa
   **todas** as outras por gRPC (`ReportStatus`).
5. **Só a mestre age.** Se não há banco primário e a réplica está viva, a
   mestre promove a réplica. Assim duas APIs nunca tentam promover ao mesmo
   tempo.

## Arquivos

```
proto/rifa.proto             ← novo serviço Monitor (Ping, ReportStatus)
internal/monitor/
├── schedule.go              ← a conta dos horários (+ teste)
└── monitor.go               ← checagem, avisos, hierarquia, promoção
internal/peerconn/           ← conexão gRPC com reconexão rápida
cmd/apinode/main.go          ← liga o monitor e a rota GET /cluster
```

## A conta dos horários (`schedule.go`)

```
período = 2 × número de APIs     → 3 APIs = 6 segundos
segundo = 2 × ID − 1             → API 1 = 1, API 2 = 3, API 3 = 5
```

Se um dia tiverem 5 APIs, a conta continua funcionando: período de 10s, e as
APIs checam nos segundos 1, 3, 5, 7 e 9.

O teste `TestSegundosCombinados` confere exatamente a tabela acima.

## O que acontece quando a API 1 (mestre) cai

```
00:54:34  docker compose stop api1
00:54:35  API 3 (sua vez, segundo 35) faz Ping na API 1 → timeout
00:54:36  API 3: "DESCOBRI: api1 up -> down. Avisando as outras APIs."
00:54:36  API 2: "a api3 me avisou: api1 up -> down"
00:54:36  API 2 e API 3: "a API mestre agora é a api2"
```

Enquanto isso, o site continua funcionando: o nginx percebe que a API 1 não
responde e manda as requisições para a 2 e a 3.

## O que acontece quando o db1 (primário) cai

```
00:54:48  docker compose stop db1
00:54:50  compras dão 503 (banco indisponível)
00:54:51  API 2 (mestre): "DESCOBRI: db1 primary -> down"
00:54:52  API 2: "SOU A MESTRE e não há banco primário: promovendo db2"
00:54:52  API 2: "db2 agora é o banco PRIMÁRIO" → avisa as outras
00:54:52  todas: "[banco] o primário mudou: reabrindo as conexões"
00:54:53  compras voltam a dar 201
```

Fora do ar por **uns 3 segundos**. Antes de promover, a mestre checa os
bancos de novo na hora, porque promover não tem volta e ela não confia só no
que os outros avisaram.

## Os detalhes que fazem funcionar

- **"unknown" conta como vivo.** Quando a API 2 sobe, ela ainda não checou
  a API 1. Se tratasse "não sei" como "caiu", ela se declararia mestre por
  engano.
- **Reconexão rápida** (`peerconn.Dial`). Por padrão, o gRPC espera cada vez
  mais entre tentativas de reconexão (até 2 minutos). Assim, uma API que
  voltou parecia "fora do ar" por muito tempo. Limitei essa espera a 1s.
- **Não avisa quem está fora do ar.** Quando a API volta, ela checa sozinha
  e descobre o estado atual.
- **Ninguém aceita aviso de que ela mesma caiu.** Se a API 1 recebe
  "api1 caiu", ela ignora, porque está respondendo.

## Ver o que cada API acha: `GET /cluster`

```bash
curl http://localhost:8082/cluster
```
```json
{
  "eu": 2, "mestre": 1, "meus_segundos": [3, 9, 15, ...],
  "ultima_checagem": "2026-11-26T00:53:31Z",
  "apis":   [{"nome": "api1", "estado": "up"}, ...],
  "bancos": [{"nome": "db1", "estado": "primary"}, {"nome": "db2", "estado": "replica"}]
}
```

O `tools/painel-teste.html` mostra isso das 3 APIs lado a lado, atualizando a
cada segundo.

## Limitação conhecida (bom citar na apresentação)

Se a **rede** entre as APIs e o db1 cair, mas o db1 continuar vivo, a mestre
promove o db2 e passam a existir dois primários (split brain). Sistemas de
verdade resolvem isso com *fencing*, ou seja, garantindo que o primário antigo
foi desligado antes de promover. Ferramentas como Patroni fazem isso. Para o
escopo do trabalho, a queda do container é o cenário testado.
