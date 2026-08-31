#!/usr/bin/env bash
# Boots vessel, seeds real Apple container CLI state, records a VHS tape of a
# genuine session, and converts the capture into the README demo assets.
#
# Usage: scripts/record-demo/record.sh   (or `make demo`)
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

for bin in vhs ffmpeg container; do
	if ! command -v "$bin" >/dev/null 2>&1; then
		echo "error: '$bin' not found in PATH (brew install $bin)" >&2
		exit 1
	fi
done

echo "==> building vessel"
go build -o vessel .

echo "==> seeding demo containers/images/volumes"
scripts/record-demo/seed.sh

echo "==> recording with vhs"
rm -f scripts/record-demo/out.mp4
vhs scripts/record-demo/demo.tape

raw="scripts/record-demo/out.mp4"
mp4="docs/assets/demo.mp4"
gif="docs/assets/demo.gif"
palette_dir="$(mktemp -d)"
trap 'rm -rf "$palette_dir"' EXIT
palette="$palette_dir/palette.png"

echo "==> encoding docs/assets/demo.mp4 (h264, yuv420p, 1280 wide)"
ffmpeg -y -i "$raw" -vf "scale=1280:-2" -pix_fmt yuv420p -c:v libx264 -movflags +faststart "$mp4"

echo "==> generating palette for gif"
ffmpeg -y -i "$raw" -vf "fps=12,scale=960:-2:flags=lanczos,palettegen" -update 1 -frames:v 1 "$palette"

echo "==> encoding docs/assets/demo.gif (960 wide, 12fps)"
ffmpeg -y -i "$raw" -i "$palette" -filter_complex "fps=12,scale=960:-2:flags=lanczos[x];[x][1:v]paletteuse" "$gif"

echo "==> sizes"
ls -lh "$mp4" "$gif"

gif_bytes=$(stat -f%z "$gif" 2>/dev/null || stat -c%s "$gif")
if [ "$gif_bytes" -gt 10485760 ]; then
	echo "error: demo.gif is $((gif_bytes / 1024 / 1024)) MiB, over the 10 MiB budget" >&2
	exit 1
fi

echo "==> done: $mp4 and $gif"
