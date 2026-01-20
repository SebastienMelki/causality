#!/bin/bash

set -e

AUTH_DIR="/app/redash/authentication"

if [ -e "$AUTH_DIR" ] && [ ! -d "$AUTH_DIR" ]; then
    timestamp=$(date +%s)
    mv "$AUTH_DIR" "${AUTH_DIR}.conflict.${timestamp}"
fi

if [ ! -d "$AUTH_DIR" ]; then
    mkdir -p "$AUTH_DIR"
fi

echo "Starting Redash Server with Auto-Setup..."

# Run initialization in background after server starts
{
    sleep 10
    echo "Running Redash auto-initialization..."
    /app/init-admin.sh
} &

# Start the original Redash server command
exec /app/bin/docker-entrypoint server
