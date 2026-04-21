#!/usr/bin/with-contenv bashio

export PORT=8099
export DATA_DIR=/data
export SUPERVISOR_TOKEN="${SUPERVISOR_TOKEN}"

INGRESS_PATH=$(bashio::addon.ingress_entry 2>/dev/null || true)
if [ -n "${INGRESS_PATH}" ]; then
    export INGRESS_PATH="${INGRESS_PATH}"
fi

SHOPPING_WEBHOOK_ID=$(bashio::config 'shopping_webhook_id' 2>/dev/null || true)
if [ -n "${SHOPPING_WEBHOOK_ID}" ]; then
    export SHOPPING_WEBHOOK_SLUG="/api/webhook/${SHOPPING_WEBHOOK_ID}"
fi

GEMINI_API_KEY=$(bashio::config 'gemini_api_key' 2>/dev/null || true)
if [ -n "${GEMINI_API_KEY}" ]; then
    export GEMINI_API_KEY="${GEMINI_API_KEY}"
fi

BASE_SERVINGS=$(bashio::config 'base_servings' 2>/dev/null || true)
if [ -n "${BASE_SERVINGS}" ]; then
    export BASE_SERVINGS="${BASE_SERVINGS}"
fi

bashio::log.info "Starting Béilí on port ${PORT} (ingress: ${INGRESS_PATH:-none})"

exec /usr/bin/server
