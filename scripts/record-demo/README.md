# Recording the demo

Regenerates `docs/assets/demo.mp4` and `docs/assets/demo.gif` from a real,
live vessel session against Apple's `container` CLI. Nothing here is
hand-authored or staged - every frame is a genuine capture.

## Run it

```
make demo
```

or directly:

```
scripts/record-demo/record.sh
```

This:

1. Builds `./vessel` from the current source.
2. Runs `seed.sh`, which starts the container system if needed and
   (re)creates a deterministic set of demo containers (`web` running nginx,
   `cache` and `worker` running alpine), two images, and a `demo-data`
   volume. It leaves `web` stopped so the recording has something to
   start/stop live. Safe to re-run - existing demo resources are deleted
   and recreated first.
3. Records `demo.tape` with [VHS](https://github.com/charmbracelet/vhs)
   (`brew install vhs`) into `out.mp4`.
4. Encodes `docs/assets/demo.mp4` (H.264, `yuv420p`, 1280px wide) and
   `docs/assets/demo.gif` (960px wide, 12fps, palette-based) with ffmpeg,
   and fails if the gif exceeds 10 MiB.

## Editing the tape

`demo.tape` drives real keypresses against the real TUI - there is no mock
mode. If you change the flow, re-run `make demo` and sanity check the
output with `ffprobe` (dimensions, frame count, duration) plus a manual
frame extraction, e.g.:

```
ffmpeg -i docs/assets/demo.gif -vf "select='not(mod(n\,40))'" -fps_mode vfr frame_%02d.png
```

Note: a container running a bare `sleep` command can take several seconds
to actually stop (it may not react to SIGTERM promptly), which reads as a
hang on camera even though vessel's own action completes instantly. nginx
(the `web` container) stops within ~100ms, which is why it's the one used
for the live start/stop beat.
