#!/bin/sh
set -eu

# Older deployments may have a model volume created by the former root-run
# parser container. Repair ownership before dropping privileges so the managed
# parser can update the Hugging Face cache.
if [ "$(id -u)" = "0" ]; then
    mkdir -p /models/huggingface /tmp/uv-cache
    chown -R memora:memora /models/huggingface /tmp/uv-cache
    exec gosu memora "$@"
fi

exec "$@"
