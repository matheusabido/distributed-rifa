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

copy_from_peer() {
    gosu postgres pg_basebackup -h "$PEER_HOST" -U replicator -D "$PGDATA" -X stream -R
}

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
    log "environment is already set up"
fi

exec docker-entrypoint.sh postgres "$@"