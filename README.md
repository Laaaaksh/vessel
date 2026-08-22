<div align="center">

<img src="docs/assets/vessel-banner.svg" alt="vessel" width="640">

**vessel** — why run many commands when few keys do the trick?

A keyboard-driven terminal UI for [Apple's native Mac containers](https://github.com/apple/container)
(the OCI-compatible containers introduced in macOS at WWDC 2025). Live lists, logs, shells,
images, and volumes - one screen, zero daemons, no CLI flags to memorize.

[![Star this repo](https://img.shields.io/github/stars/Laaaaksh/vessel?style=for-the-badge&logo=github&label=star%20this%20repo&color=yellow)](https://github.com/Laaaaksh/vessel/stargazers)
[![Built for Apple container](https://img.shields.io/badge/built_for-Apple_Container-00ADD8?style=for-the-badge&logo=docker&logoColor=white)](https://github.com/apple/container)

[![CI](https://github.com/Laaaaksh/vessel/actions/workflows/ci.yml/badge.svg)](https://github.com/Laaaaksh/vessel/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Laaaaksh/vessel?color=green&display_name=tag)](https://github.com/Laaaaksh/vessel/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platform](https://img.shields.io/badge/platform-macOS%2026%2B%20%E2%80%A2%20Apple%20silicon-000?logo=apple&logoColor=white)](#requirements)
[![Homebrew](https://img.shields.io/badge/brew-Laaaaksh%2Fvessel-orange?logo=homebrew)](#install)

**[Install](#install) • [Usage](#usage) • [Keybindings](#keybindings) • [Configuration](#configuration) • [CLI matrix](docs/APPLE_CONTAINER_MATRIX.md) • [Contributing](CONTRIBUTING.md) • [License](LICENSE)**

**[Code of conduct](CODE_OF_CONDUCT.md) • [Contributing](CONTRIBUTING.md) • [License](LICENSE) • [Security](SECURITY.md)**

</div>

## What it does

`vessel` gives you a keyboard-driven dashboard for managing Mac containers - the OCI-compatible native containers introduced in macOS at WWDC 2025. No more memorizing CLI flags or running multiple commands to see what's running.

- Live container list with status, CPU %, memory, and sparklines
- Start, stop, restart, remove, and prune
- Drop into a shell inside any running container (clean UI restore on exit)
- Stream logs with follow freeze and in-buffer search
- Inspect ports, env, labels, and details
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
| `s` / `u` / `r` | Stop / start / restart |
| `d` | Remove (confirm with `y`) |
| `space` | Toggle multi-select mark |
| `/` | Filter current list |
| `y` | Yank id / name / path |
| `x` | Action menu |
| `p` | Pull image (images view) |
| `P` | Prune (stopped containers / images / volumes) |
| `c` | Create / run (prompt) |
| `+` / `_` | Cycle layout |
| `` ` `` | Toggle command log |
| `tab` / `1` `2` `3` | Containers / Images / Volumes |
| `esc` | Close logs, help, modal, or clear filter |
| `?` | Toggle help |
| `q` | Quit |

## Configuration

vessel reads `~/.config/vessel/config.toml` (see `config.example.toml`):

```toml
poll_interval = "2s"
log_tail_lines = 100
mouse_enabled = true
shell = "/bin/sh"

# [[custom_commands]]
# name = "inspect"
# command = "container inspect {{.ID}}"
```

## Apple CLI matrix

Live probe notes for Apple `container` 1.2.x live in [`docs/APPLE_CONTAINER_MATRIX.md`](docs/APPLE_CONTAINER_MATRIX.md).

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) - all contributors must sign the CLA before their first PR is merged.

## Security

Found a security issue? Please report it privately - see [SECURITY.md](SECURITY.md).

## Star this repo

If `vessel` makes managing containers on your Mac easier, [leave a star](https://github.com/Laaaaksh/vessel/stargazers) - it helps other people find it.

[![Star History Chart](https://api.star-history.com/svg?repos=Laaaaksh/vessel&type=Date)](https://star-history.com/#Laaaaksh/vessel&Date)

## License

Apache 2.0 - see [LICENSE](LICENSE).
