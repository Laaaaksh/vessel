# Contributing to vessel

Thank you for your interest in contributing. vessel is open source under the MIT license.

## Contributor License Agreement (CLA)

Before your first pull request is merged, you must sign the Contributor License Agreement.
The CLA bot will comment on your PR with instructions when you open one.

The CLA grants the project maintainer the right to use your contribution under any license,
including for future commercialization. This is standard practice for open-source projects
that may be acquired or relicensed.

## Getting started

```bash
git clone https://github.com/Laaaaksh/vessel.git
cd vessel
go mod download
make build
make test
```

## Requirements

- Go 1.26+
- The runtime requirements in [README.md](README.md#requirements) (macOS and the `container` CLI),
  needed only for the live tests below.

## Development workflow

1. Fork the repo and create a feature branch from `main`.
2. Make your changes with tests.
3. Run `make lint` and `make test` - both must pass.
4. If your change is user-facing (a feature, fix, or behavior change), add one
   bullet under the `Unreleased` heading in [CHANGELOG.md](CHANGELOG.md).
5. Open a PR against `main`. The CLA bot will prompt you to sign the CLA if you haven't already.

`make test` runs against the fake `container` CLI in `internal/backend/fakecli/`, so it needs
no runtime. To exercise the real Apple CLI:

```bash
go test -tags=live ./internal/backend -run Live -v
./scripts/smoke.sh   # unit tests, plus the live tests if `container` is available
```

## Releases

Releases are cut by pushing a tag; GitHub Actions does the rest
(`.github/workflows/release.yml`):

1. Make sure every user-facing change since the last release has a bullet under
   `Unreleased` in [CHANGELOG.md](CHANGELOG.md) (step 4 of the workflow above).
2. Give the release its own changelog section: insert `## [x.y.z] - YYYY-MM-DD`
   above the (now empty) `## [Unreleased]` heading, following the format of the
   existing sections, and update the compare links at the bottom of the file -
   add `[x.y.z]: https://github.com/Laaaaksh/vessel/compare/v<prev>...vx.y.z`
   and repoint `[Unreleased]` at `compare/vx.y.z...HEAD`.
3. Commit those changelog edits to `main`, then tag and push:

   ```bash
   git tag vx.y.z && git push origin vx.y.z
   ```

The workflow extracts the tagged version's CHANGELOG section as the GitHub
release notes (`scripts/release_notes.sh`: if the version has no heading yet it
falls back to the `Unreleased` bullets, and it fails the release rather than
publishing empty notes). GoReleaser - pinned to an exact version, see the
workflow - then builds the Apple-silicon tarball and checksums, publishes the
release with those notes, and updates the Homebrew tap formula. The tag itself
becomes the binary's self-reported version (`vessel --version`).

## Code style

- Standard `gofmt` / `goimports` formatting (enforced by CI).
- Follow the existing package structure: TUI code in `internal/ui/`, backend in `internal/backend/`.
- Keep `Update()` and `View()` functions thin - delegate to helper functions per screen.
- Never use raw ANSI codes - use lipgloss styles defined in `internal/ui/styles.go`.

## Reporting issues

Please include:
- macOS version
- `container --version` output
- Steps to reproduce
- What you expected vs what happened
