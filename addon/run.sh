#!/bin/bash
set -e

export PORT=8099
export DATA_DIR=/data

echo "Starting Béilí on port ${PORT}"
exec /usr/bin/server
