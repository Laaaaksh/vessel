# vessel

A keyboard-driven terminal UI for Apple's native Mac containers.

```
brew tap Laaaaksh/vessel
brew install vessel
```

## What it does

`vessel` gives you a keyboard-driven dashboard for managing Mac containers - the OCI-compatible native containers introduced in macOS at WWDC 2025. No more memorizing CLI flags or running multiple commands to see what's running.

- Live container list with status, CPU %, memory, and sparklines
- Start, stop, restart, remove, and prune
- Drop into a shell inside any running container (clean UI restore on exit)
- Stream logs with follow freeze and in-buffer search
- Inspect containers: ports, mounts, networks and IP, CPUs, memory, platform, hostname, env, labels
- Inspect images (digest, layers, command, platform variants) and volumes (quota, format, labels, options)
- Browse / pull / prune images; tag, save, load, and push them; create / prune volumes
- Filter on every list; multi-select; action menu; custom commands
- Vim-style navigation, pane focus, mouse click/wheel

## Requirements

- macOS 26+ on Apple silicon (macOS 15 may work with limitations)
- [Apple Container CLI](https://github.com/apple/container) in your PATH

```bash
brew install container
container system start   # downloads a default Linux kernel on first run
```

## Install

```bash
brew tap Laaaaksh/vessel
brew install vessel
```

Or download a binary from [GitHub Releases](https://github.com/Laaaaksh/vessel/releases).

## Usage

```bash
vessel
vessel doctor   # check CLI, system status, config
```

### Keybindings

| Key | Action |
|-----|--------|
| `h` / `l` / `←` / `→` | Move focus (sidebar / list / detail) |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` / `G` | Go to top / bottom |
| `pgup` / `pgdn` / `ctrl+u` / `ctrl+d` | Page scroll |
| `enter` | Open shell in container |
| `L` | View logs |
| `f` | Freeze / follow logs |
| `s` / `u` / `r` | Stop / start / restart (stop asks to confirm when `confirm_stop` is set) |
| `d` | Remove the selected row, or every marked row when 2+ are marked (confirm with `y`) |
| `space` | Toggle multi-select mark (containers, images, volumes) |
| `/` | Filter current list |
| `y` | Yank id / name / path |
| `x` | Action menu |
| `p` | Pull image (images view) |
| `P` | Prune (stopped containers / images / volumes), confirm with `y` |
| `c` | Create / run (prompt) |
| `+` / `_` | Cycle layout |
| `` ` `` | Toggle command log |
| `tab` / `1` `2` `3` | Containers / Images / Volumes |
| `esc` | Close logs, help, modal, or clear filter |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

A custom command with a `key` set fires on that key and replaces the built-in action on it, except on reserved keys (navigation, filtering, and the global keys) - `config.example.toml` documents which keys can be taken over. The in-app help (`?`) always lists what each key currently does.

### Images: tag, save, load, push

Pick an image, press `x`, and choose `Tag…`, `Save…`, `Load…` or `Push`. Tag, save and
push need a named reference, so an untagged row is refused rather than quietly resolved
to `:latest`. Save prompts for an archive path and confirms before overwriting a file
that already exists; load prompts for an existing archive and says so plainly when the
path is missing; push confirms first, because it publishes.

Known limits:

- vessel never manages registry credentials. Push reuses whatever session
  `container registry login` has already established - running that login is yours to do.
- Long-running verbs get real budgets: image tag/save/load/push, prunes and starting a
  container run up to two minutes, one batched delete of many targets gets one minute for
  the whole call, and a one-shot exec gets thirty seconds.
  A huge `image pull` is still bounded by the short default cap.
- The prompt drops the space bar and non-ASCII characters, so a path containing either
  cannot be typed yet - save would write somewhere you did not name, and load reports a
  missing file for one that exists.

## Configuration

vessel reads `~/.config/vessel/config.toml` (see `config.example.toml`):

```toml
poll_interval = "2s"
log_tail_lines = 100
mouse_enabled = true
shell = "/bin/sh"

# [[custom_commands]]
# name = "inspect"
# key = "z"          # optional: "z", "space", "enter", "f5", "ctrl+z"
# command = "container inspect {{.ID}}"
```

## Apple CLI matrix

Live probe notes for Apple `container` 1.2.x live in [`docs/APPLE_CONTAINER_MATRIX.md`](docs/APPLE_CONTAINER_MATRIX.md).

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) - all contributors must sign the CLA before their first PR is merged.

## License

MIT - see [LICENSE](LICENSE).
