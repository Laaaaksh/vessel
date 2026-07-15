# vessel

A keyboard-driven terminal UI for Apple's native Mac containers.

```
brew tap Laaaaksh/vessel
brew install vessel
```

## What it does

`vessel` gives you a keyboard-driven dashboard for managing Mac containers - the OCI-compatible native containers introduced in macOS at WWDC 2025. No more memorizing CLI flags or running multiple commands to see what's running.

- Live container list with status, CPU %, and memory
- Start, stop, restart, and remove containers
- Drop into a shell inside any running container
- Stream logs in real time
- Inspect port mappings, environment variables, and container details
- Filter containers by name or image
- Vim-style navigation (j/k, g/G)

## Requirements

- macOS 15+ (Apple silicon or Intel)
- Apple Mac containers installed (the `container` CLI must be in your PATH)

## Install

```bash
brew tap Laaaaksh/vessel
brew install vessel
```

Or download a binary from [GitHub Releases](https://github.com/Laaaaksh/vessel/releases).

## Usage

```bash
vessel
```

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` / `G` | Go to top / bottom |
| `enter` | Open shell in container |
| `L` | View logs |
| `s` | Stop container |
| `u` | Start container |
| `r` | Restart container |
| `d` | Remove container |
| `/` | Filter containers |
| `?` | Toggle help |
| `q` | Quit |

## Configuration

vessel reads `~/.config/vessel/config.toml`:

```toml
poll_interval = "2s"   # how often to refresh metrics
log_tail_lines = 100   # lines to show in log tail
mouse_enabled = true
```

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) - all contributors must sign the CLA before their first PR is merged.

## License

Apache 2.0 - see [LICENSE](LICENSE).
