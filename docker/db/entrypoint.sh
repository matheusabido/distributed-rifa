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

log "starting $MY_HOST, connecting to $PEER_HOST"

peer_state=0
check_peer || peer_state=$?

log "peer state: $peer_state"

exec docker-entrypoint.sh postgres "$@"