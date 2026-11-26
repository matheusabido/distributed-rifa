#!/bin/bash
# Entrypoint dos bancos da rifa. Decide, ANTES de ligar o Postgres, se este
# container vai ser PRIMÁRIO ou RÉPLICA, e depois chama o entrypoint oficial.
#
# Variáveis (vêm do docker-compose):
#   NODE_NAME      nome deste banco (db1, db2)       -> só para os logs
#   PEER_HOST      o outro banco                      -> ex: db2
#   INITIAL_ROLE   primary ou standby                 -> papel na PRIMEIRA vez
#   POSTGRES_PASSWORD                                 -> também é a senha do replicator
#
# Regras:
#   1. Primeira vez, papel "primary": cria o banco do zero (initdb + scripts
#      de /docker-entrypoint-initdb.d: usuário replicator e tabelas).
#   2. Primeira vez, papel "standby": espera o outro virar primário e copia
#      tudo dele (pg_basebackup). O -R deixa configurado para seguir copiando.
#   3. Reinício de quem era primário, mas o outro virou primário enquanto
#      ele estava fora: os dados dele estão velhos. Apaga e copia do novo
#      primário, voltando como RÉPLICA ("o mestre antigo volta como réplica").
#   4. Qualquer outro caso: liga normalmente.
set -e

export PGPASSWORD="$POSTGRES_PASSWORD"
export PGCONNECT_TIMEOUT=2
NODE="${NODE_NAME:-db}"

log() { echo "[$NODE] $*"; }

# O outro banco está de pé E aceita escrita (não está em recovery)?
peer_is_primary() {
  [ "$(psql -h "$PEER_HOST" -U postgres -d postgres -tAc 'select pg_is_in_recovery()' 2>/dev/null)" = "f" ]
}

clone_from_peer() {
  log "copiando todos os dados de $PEER_HOST (pg_basebackup) e virando RÉPLICA"
  rm -rf "$PGDATA"
  mkdir -p "$PGDATA"
  chown postgres:postgres "$PGDATA"
  chmod 700 "$PGDATA"
  gosu postgres pg_basebackup -h "$PEER_HOST" -U replicator -D "$PGDATA" -X stream -R
  log "cópia pronta"
}

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  # --- primeira vez (pasta de dados vazia) ---
  if [ "$INITIAL_ROLE" = "standby" ]; then
    until peer_is_primary; do
      log "esperando $PEER_HOST ficar pronto como primário..."
      sleep 2
    done
    clone_from_peer
  else
    log "primeira vez: criando o banco como PRIMÁRIO"
  fi
else
  # --- reinício ---
  if [ ! -f "$PGDATA/standby.signal" ] && peer_is_primary; then
    log "eu era primário, mas $PEER_HOST virou primário enquanto eu estava fora"
    clone_from_peer
  elif [ -f "$PGDATA/standby.signal" ]; then
    log "reiniciando como RÉPLICA"
  else
    log "reiniciando como PRIMÁRIO"
  fi
fi

exec docker-entrypoint.sh "$@"
