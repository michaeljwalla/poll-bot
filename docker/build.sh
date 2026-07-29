#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

GO_VER=$(awk '/^go [0-9]/ {print $2}' go.mod) 
docker build \
    --build-arg GO_VER=$GO_VER \
    -t "poll-bot" \
    .