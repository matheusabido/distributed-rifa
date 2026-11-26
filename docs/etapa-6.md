# Etapa 6: dois bancos (primário + réplica) e datas em UTC

## Os dois bancos

A base é o Postgres 19 do seu amigo (`docker/db/`: `Dockerfile`,
`postgresql.conf` com `wal_level = replica`, `pg_hba.conf` e o usuário
`replicator`). Faltava fazer o db2 **copiar** o db1, e foi isso que eu
completei.

```
db1 (primário) ──── streaming replication ────► db2 (réplica)
aceita escrita        (cada mudança é enviada    só leitura, cópia
                       na hora para a réplica)    sempre atualizada
```

### Quem decide o papel de cada um: `docker/db/rifa-entrypoint.sh`

Antes de ligar o Postgres, esse script olha a situação:

| Situação | O que acontece |
|---|---|
| 1ª vez, `INITIAL_ROLE=primary` (db1) | cria o banco do zero: usuário `replicator` + tabelas (`db/schema.sql`) |
| 1ª vez, `INITIAL_ROLE=standby` (db2) | espera o db1 ficar pronto e copia tudo dele com `pg_basebackup -R` |
| reinício de quem era primário, mas o outro virou primário enquanto ele estava fora | apaga os dados velhos, copia do novo primário e volta como **réplica** |
| qualquer outro reinício | liga normalmente, no papel que tinha |

O `-R` do `pg_basebackup` cria o arquivo `standby.signal` e grava onde está o
primário. É isso que faz o Postgres ligar como réplica e continuar copiando.

### Por que quem volta precisa virar réplica

Se o db1 voltasse como primário, existiriam **dois primários** com dados
diferentes: as compras feitas no db2 enquanto o db1 estava fora não existem
no db1. Isso se chama *split brain* e é o pior erro num banco distribuído.
Por isso o script apaga e copia do primário atual.

### Quem promove o db2 quando o db1 cai?

O banco não se promove sozinho. Quem faz isso é a **API mestre**, rodando
`SELECT pg_promote()` no db2 (etapa 7).

### Como a API acha o primário

A conexão lista os dois bancos:

```
postgres://postgres:***@db1:5432,db2:5432/rifa?target_session_attrs=read-write
```

O pgx tenta db1, depois db2, e fica com o que **aceita escrita**. Quando o
primário muda, a API fecha as conexões antigas (`store.Reset()`) e as novas
vão sozinhas para o lugar certo.

### Ver a replicação funcionando

```bash
# no primário: quem está copiando de mim?
docker compose exec db1 psql -U postgres -c "select client_addr, state from pg_stat_replication"
# em cada um: sou réplica? (t = réplica, f = primário)
docker compose exec db2 psql -U postgres -tAc "select pg_is_in_recovery()"
```

## Datas sempre em UTC

- As colunas `created_at` e `bought_at` agora são **`TIMESTAMP` sem fuso**.
- O padrão é `now() AT TIME ZONE 'utc'`: pega a hora atual já em UTC,
  qualquer que seja o fuso do servidor.
- A API também força cada conexão a usar `timezone=UTC`
  (`RuntimeParams` em `store.Connect`).
- Na resposta JSON, as datas saem com `Z` no final (`2026-11-26T00:54:23Z`),
  que é o jeito padrão de dizer "isto é UTC". Quem converte para o horário
  local de quem está vendo é o frontend.

Teste: configurei um banco com fuso `America/Cuiaba` e o `TestFluxoCompleto`
confere que `criada_em` continua em UTC e bate com a hora atual.

## Outras mudanças na API

- **A API sobe mesmo sem banco.** Antes ela encerrava se o banco não
  respondesse. Agora só avisa no log e tenta de novo a cada requisição.
- **503 em vez de 500** quando o banco está fora do ar ou trocando de
  primário: `{"erro": "banco de dados indisponível, tente de novo em instantes"}`.
