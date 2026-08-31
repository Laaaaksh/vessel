#!/usr/bin/env bash
# Seeds a deterministic set of demo containers, images, and a volume for the
# vessel demo recording. Safe to re-run: existing demo resources are removed
# and recreated so every take starts from the same state.
#
# Resources are named with a vessel-demo- prefix, not generic names like
# "web"/"cache"/"worker": those are common names a developer might already be
# using for their own containers, and this script destroys whatever it finds
# under the names it manages. See teardown.sh for cleanup.
set -euo pipefail

if ! command -v container >/dev/null 2>&1; then
	echo "error: Apple 'container' CLI not found in PATH" >&2
	exit 1
fi

if ! container system status >/dev/null 2>&1; then
	echo "starting container system..."
	container system start
fi

echo "seeding demo containers, images, and volume..."

container stop vessel-demo-worker vessel-demo-cache vessel-demo-web >/dev/null 2>&1 || true
container delete vessel-demo-worker vessel-demo-cache vessel-demo-web >/dev/null 2>&1 || true
container volume delete vessel-demo-data >/dev/null 2>&1 || true

container image pull docker.io/library/alpine:latest >/dev/null
container image pull docker.io/library/nginx:alpine >/dev/null

# 18080, not 8080/8081/8090: those are common defaults for other local
# tooling (Docker Desktop, dev servers) and a collision fails the whole seed
# with a "bind: address already in use" error from the container runtime.
container run -d --name vessel-demo-web -p 18080:80 docker.io/library/nginx:alpine >/dev/null
container run -d --name vessel-demo-cache docker.io/library/alpine:latest sleep 3600 >/dev/null
container run -d --name vessel-demo-worker docker.io/library/alpine:latest sleep 3600 >/dev/null
container volume create vessel-demo-data >/dev/null

# Leave the nginx container stopped so the demo shows a mixed running/stopped
# list and has something to start/stop live on camera. nginx reacts to
# SIGTERM immediately; a plain "sleep" container can take several seconds to
# actually exit, which reads as a hang on screen even though vessel's side is
# instant.
container stop vessel-demo-web >/dev/null

echo "seed complete:"
container list --all
container image list
container volume list
