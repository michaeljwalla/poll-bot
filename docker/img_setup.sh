#!/bin/sh

#subject to more change so outside dockerfile
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR

mkdir -p ./data/logs/
touch ./data/aliases.json