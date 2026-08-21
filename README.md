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
- Browse / pull / prune images; create / prune volumes
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
| `q` | Quit |

A custom command with a `key` set fires on that key and replaces the built-in action on it, except on reserved keys (navigation, filtering, and the global keys) - `config.example.toml` documents which keys can be taken over. The in-app help (`?`) always lists what each key currently does.

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
