#!/usr/bin/env bash
# Removes the demo containers, images, and volume that seed.sh creates.
# Safe to re-run; missing resources are ignored.
set -euo pipefail

if ! command -v container >/dev/null 2>&1; then
	echo "error: Apple 'container' CLI not found in PATH" >&2
	exit 1
fi

echo "tearing down demo containers, images, and volume..."

container stop vessel-demo-worker vessel-demo-cache vessel-demo-web >/dev/null 2>&1 || true
container delete vessel-demo-worker vessel-demo-cache vessel-demo-web >/dev/null 2>&1 || true
container volume delete vessel-demo-data >/dev/null 2>&1 || true
container image delete docker.io/library/alpine:latest >/dev/null 2>&1 || true
container image delete docker.io/library/nginx:alpine >/dev/null 2>&1 || true

echo "teardown complete."
