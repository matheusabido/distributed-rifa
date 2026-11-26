#!/bin/bash
set -e

export PGCONNECT_TIMEOUT=3

log() { echo "[entrypoint] $*"; }

check_peer() {
  local r
  r=$(psql -h "$PEER_HOST" -U postgres -d postgres -tAc \
      "SELECT pg_is_in_recovery()" 2>/dev/null) || return 2
  [ "$r" = "f" ]
}

QUERY="SELECT CASE WHEN pg_is_in_recovery() THEN 'replica' ELSE 'master' END,
              timeline_id FROM pg_control_checkpoint()"

peer_info() {
    psql -h "$PEER_HOST" -U postgres -d postgres -tA -F' ' -c "$QUERY" 2>/dev/null
}

my_info() {
    psql -U postgres -d postgres -tA -F' ' -c "$QUERY" 2>/dev/null
}

disk_timeline() {
  gosu postgres pg_controldata "$PGDATA" \
    | grep "^Latest checkpoint's TimeLineID:" \
    | awk '{print $NF}'
}

copy_from_peer() {
  gosu postgres pg_basebackup -h "$PEER_HOST" -U replicator -D "$PGDATA" -X stream -R
}

stop_postgres() {
  gosu postgres pg_ctl stop -D "$PGDATA" -m fast || true
}

log "===== boot $(date '+%H:%M:%S') ====="
log "setting up file system"

mkdir -p "$PGDATA"
chown postgres:postgres "$PGDATA"
chmod 700 "$PGDATA"

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  if [ "$MY_HOST" = "$INITIAL_MASTER" ]; then
    log "empty disk and initial master: creating new db instance"
  else
    until check_peer; do
      log "waiting for $PEER_HOST to be a master before setting up db"
      sleep 2
    done
    log "empty disk and replica, copying from master"
    copy_from_peer
  fi
else
  read -r peer_role peer_tl <<< "$(peer_info || true)"
  my_tl=$(disk_timeline)
  log "disk has data; my timeline: $my_tl | $PEER_HOST: ${peer_role:-down} $peer_tl"

  if [ "$peer_role" = "master" ] && [ "$peer_tl" -ge "$my_tl" ]; then
    log "$PEER_HOST is the master; becoming a replica"
    if ! gosu postgres pg_rewind -R --target-pgdata="$PGDATA" \
          --source-server="host=$PEER_HOST user=postgres dbname=postgres"; then
      log "pg_rewind failed: wiping and copying from scratch"
      rm -rf "${PGDATA:?}"/*
      copy_from_peer
    fi
  elif [ "$peer_role" = "master" ]; then
    log "$PEER_HOST is master but older (timeline $peer_tl < $my_tl): I'm the real master"
  elif [ -f "$PGDATA/standby.signal" ]; then
    log "I'm a replica and $PEER_HOST is not master: staying replica"
  else
    log "I was master and $PEER_HOST is not master: staying master"
  fi
fi

log "starting postgres"
docker-entrypoint.sh postgres "$@" &
PG=$!

trap 'log "stop requested"; stop_postgres' INT TERM

watchdog() {
    sleep 20
    while kill -0 "$PG" 2>/dev/null; do
        sleep 5

        read -r me my_tl <<< "$(my_info || true)"
        [ "$me" = "master" ] || continue

        read -r peer peer_tl <<< "$(peer_info || true)"
        [ "$peer" = "master" ] || continue

        if [ "$peer_tl" -gt "$my_tl" ] ||
           { [ "$peer_tl" -eq "$my_tl" ] && [ "$MY_HOST" != "$INITIAL_MASTER" ]; }; then
            log "TWO MASTERS: $PEER_HOST (timeline $peer_tl) wins over me ($my_tl). Stopping."
            stop_postgres
            return
        fi
        log "TWO MASTERS: I (timeline $my_tl) win over $PEER_HOST ($peer_tl). It will yield."
    done
}
watchdog &

wait "$PG" || true
while kill -0 "$PG" 2>/dev/null; do sleep 1; done
log "postgres stopped"