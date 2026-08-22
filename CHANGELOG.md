# Changelog

All notable changes to vessel are documented in this file. Released sections mirror
the notes on the [GitHub Releases page](https://github.com/Laaaaksh/vessel/releases),
condensed into user-facing terms. Format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed
- The space bar and multi-byte characters (accents, CJK, emoji) are no longer
  silently dropped when typed into the container, image, volume and network
  filters or the log search box.
- Backspace deletes whole characters, so removing accented or multi-byte input
  can no longer corrupt the text buffer.
- `image pull` now runs under the two-minute transfer budget instead of being
  killed by the short default timeout partway through a real download.
- Pressing a write action (prune, pull, run, volume create) when the `container`
  CLI is absent reports "container CLI unavailable" instead of crashing.
- A malformed `config.toml` no longer crashes the dashboard (a bad
  `poll_interval` panicked the metrics loop) or silently run with defaults:
  the parse error is reported in the footer at startup, unsafe values are
  clamped to safe defaults, and `vessel doctor` already printed the error.
- Custom command output is no longer polluted by login-shell profile chatter:
  `bash -l` sources `~/.profile`, whose stderr noise used to be merged into
  the result text and push the command's real output out of the footer's
  truncation window. Success now shows the command's stdout alone (empty
  output still reports "custom ok"); failures carry the exit status plus the
  command's stderr diagnostics.
- The resting footer no longer advertises a built-in action on a key a custom
  command has taken over: hints are derived from the same binding resolution
  the help screen uses, so help and footer agree, and the owning command
  shows in place of the built-in it replaced.
- The README install instructions work on current Homebrew: since Homebrew
  6.0.0, third-party taps must be trusted before they can be installed from,
  so the documented flow now includes a one-time `brew trust laaaaksh/vessel`
  (verified end-to-end against Homebrew 6.0).
- Image push refusals keep their recovery hint visible at any terminal size:
  the hint used to ride at the tail of the long registry error string, so
  footer truncation could erase it entirely, and the images detail pane's
  permission notice overflowed the narrower frame the wide-list layout hands
  it. The raw error is truncated instead, so the `container registry login`
  command always stays on screen.
- Help overlays advertise only what their view can run: the read-only System
  view no longer inherits the containers verbs (stop/start/restart, logs,
  exec, shell, remove, prune, create), and every view's help states what
  `enter` does outside them (confirm dialogs, enter the list from the
  sidebar).

### Changed
- Unknown command-line arguments print usage and exit with code 2 rather than
  silently opening the dashboard.
- Corrected README/help drift: the one-shot exec action (`e`) is now documented,
  the `c` row reflects the container run form, and page scrolling is spelled the
  way the runtime spells it (`pgdown`); dropped a Known-limits bullet describing
  prompt-input behavior that no longer exists.
- Recaptured `docs/APPLE_CONTAINER_MATRIX.md` against live Apple `container`
  1.2.2 output and recorded what has been live-verified.
- Removed the never-applied `[theme]` configuration: the example block is gone
  from `config.example.toml` and the parsed-but-unused struct was deleted from
  the code. Old config files carrying a `[theme]` table still load fine (the
  key is simply ignored).
- Normalized developer home paths out of test fixtures and captured CLI output.
- The README now shows a CI status badge so workflow health is visible at a
  glance.
- The release workflow pins GoReleaser to v2.17.1 (verified end-to-end with a
  local snapshot build of this tree) instead of floating to `latest`, so a
  future upstream release that drops the deprecated Homebrew tap support can
  no longer break tagging.

### Added
- Regression test pinning every README keybinding to a help-overlay row and vice
  versa (`internal/ui/readme_parity_test.go`), so keybinding documentation
  cannot drift again.
- Regression test proving the dashboard shows a "terminal too small" hint below
  its documented 60x12 minimum size instead of rendering broken layout
  (verified live down to a 1x5 terminal).
- Community health files: Code of Conduct, bug-report and feature-request issue
  forms, pull-request template, and SECURITY.md with private vulnerability
  reporting.
- Automated dependency updates via Dependabot (Go modules and GitHub Actions),
  and the CI workflow now runs with read-only token permissions.
- Workflow hardening: superseded CI runs are cancelled automatically, release
  runs queue instead of racing on a re-pushed tag, and every job fails fast on
  a hang via an explicit timeout instead of the 6-hour runner default.
- The README links to this changelog and CONTRIBUTING.md asks contributors to
  record user-facing changes under `Unreleased`, so version history stays
  discoverable and maintained.
- Tagged releases now publish their CHANGELOG section as the GitHub release
  notes (falling back to `Unreleased` for a brand-new version) instead of
  goreleaser's raw commit list.
- CONTRIBUTING.md documents the release process end to end: turning
  `Unreleased` changelog work into a dated version section, tagging, and what
  the tag triggers (curated release notes, GoReleaser build, Homebrew formula).

### Removed
- An unreferenced personal development script left behind in `scripts/`.

## [0.2.1] - 2026-08-21

### Fixed
- The generated Homebrew tap formula now lands in the tap's `Formula/`
  directory instead of its root, so `brew install` keeps serving fresh builds
  even alongside older hand-written formulae.

## [0.2.0] - 2026-08-21

### Added
- Read-only Networks view (#36) and System view with service status and disk
  usage (#35).
- Container run/create form plus one-shot command execution inside a running
  container (#34).
- Image mobility actions in the images panel: tag, save, load and push, with
  scoped registry-auth and permission hints on failure (#33).
- Multi-select marks (space to mark) with confirmed bulk delete for images and
  volumes (#30).
- Inspect-backed detail panes: mounts, networks, resources and hostname for
  containers (#31); deep details for images and volumes (#32).

### Changed
- Relicensed vessel from Apache-2.0 to MIT (#29).

### Fixed
- Prompt input dropping spaces and multi-byte runes.
- Release pipeline goreleaser deprecations and `--version` wiring (#37).
- Footer overflow and clamping, help-screen fitting, stale confirm state,
  duplicate in-flight inspects, and detail-pane rows overflowing their budgets.

## [0.1.1] - 2026-08-05

### Fixed
- Uses `--format json` (not `--json`) for container CLI commands; removed the
  unsupported `--all` flag from stats invocations.

## [0.1.0] - 2026-08-05

### Added
- Initial release of vessel, a keyboard-driven TUI for Apple's native Mac
  containers.

[Unreleased]: https://github.com/Laaaaksh/vessel/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/Laaaaksh/vessel/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Laaaaksh/vessel/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/Laaaaksh/vessel/compare/v0.1.0...v0.1.1
