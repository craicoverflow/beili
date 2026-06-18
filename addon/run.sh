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

OURGROCERIES_EMAIL=$(bashio::config 'ourgroceries_email' 2>/dev/null || true)
if [ -n "${OURGROCERIES_EMAIL}" ]; then
    export OURGROCERIES_EMAIL="${OURGROCERIES_EMAIL}"
fi

OURGROCERIES_PASSWORD=$(bashio::config 'ourgroceries_password' 2>/dev/null || true)
if [ -n "${OURGROCERIES_PASSWORD}" ]; then
    export OURGROCERIES_PASSWORD="${OURGROCERIES_PASSWORD}"
fi

OURGROCERIES_LIST_ID=$(bashio::config 'ourgroceries_list_id' 2>/dev/null || true)
if [ -n "${OURGROCERIES_LIST_ID}" ]; then
    export OURGROCERIES_LIST_ID="${OURGROCERIES_LIST_ID}"
fi

GEMINI_API_KEY=$(bashio::config 'gemini_api_key' 2>/dev/null || true)
if [ -n "${GEMINI_API_KEY}" ]; then
    export GEMINI_API_KEY="${GEMINI_API_KEY}"
fi

GEMINI_MODEL=$(bashio::config 'gemini_model' 2>/dev/null || true)
if [ -n "${GEMINI_MODEL}" ]; then
    export GEMINI_MODEL="${GEMINI_MODEL}"
fi

BASE_SERVINGS=$(bashio::config 'base_servings' 2>/dev/null || true)
if [ -n "${BASE_SERVINGS}" ]; then
    export BASE_SERVINGS="${BASE_SERVINGS}"
fi

bashio::log.info "Starting Béilí on port ${PORT} (ingress: ${INGRESS_PATH:-none})"

exec /usr/bin/server
