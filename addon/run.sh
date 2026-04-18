#!/usr/bin/with-contenv bashio

export PORT=8099
export DATA_DIR=/data
export SUPERVISOR_TOKEN="${SUPERVISOR_TOKEN}"

INGRESS_PATH=$(bashio::addon.ingress_entry 2>/dev/null || true)
if [ -n "${INGRESS_PATH}" ]; then
    export INGRESS_PATH="${INGRESS_PATH}"
fi

bashio::log.info "Starting Béilí on port ${PORT} (ingress: ${INGRESS_PATH:-none})"

exec /usr/bin/server
