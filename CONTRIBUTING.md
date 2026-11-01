# Contributing to vessel

Thank you for your interest in contributing. vessel is a keyboard-driven terminal UI for
Apple's native Mac containers, open source under the MIT license.

## Getting started

```bash
git clone https://github.com/<your-username>/vessel.git   # your fork, see below
cd vessel
go mod download
make build
make test
```

## Requirements

- Go 1.26+
- The runtime requirements in [README.md](README.md#requirements) (macOS and the `container` CLI),
  needed only for the live tests below.

## Contribution workflow

The `main` branch is protected: every change lands through a pull request, required status
checks must pass, and protection is enforced for everyone - including the maintainer. There
are no direct pushes to `main`.

1. Fork the repo on GitHub, then clone your fork (command above).
2. Create a descriptively named feature branch from `main`.
3. Make your changes as small, focused commits, each leaving the tree buildable.
4. Run `make lint` and `make test` - both must pass.
5. If your change is user-facing (a feature, fix, or behavior change), add one
   bullet under the `Unreleased` heading in [CHANGELOG.md](CHANGELOG.md).
6. Push the branch to your fork.
7. Open a pull request against `main` here.

A PR can merge only when every required check passes (`Test`, `Lint`, and
`update-go_modules-graph`) and all conversation threads are resolved.

### Manual testing

`make test` runs against the fake `container` CLI in `internal/backend/fakecli/`, so it needs
no runtime. To exercise real container operations you need macOS 26+ on Apple silicon with
the Apple Container CLI installed:

```bash
go test -tags=live ./internal/backend -run Live -v
./scripts/smoke.sh   # unit tests, plus the live tests if `container` is available
```

UI-only contributions are welcome even without that setup - the unit tests cover everything
a lint and test run needs.

## Releases

Releases are cut by pushing a tag; GitHub Actions does the rest
(`.github/workflows/release.yml`):

1. Make sure every user-facing change since the last release has a bullet under
   `Unreleased` in [CHANGELOG.md](CHANGELOG.md) (step 5 of the workflow above).
2. Give the release its own changelog section: insert `## [x.y.z] - YYYY-MM-DD`
   above the (now empty) `## [Unreleased]` heading, following the format of the
   existing sections, and update the compare links at the bottom of the file -
   add `[x.y.z]: https://github.com/Laaaaksh/vessel/compare/v<prev>...vx.y.z`
   and repoint `[Unreleased]` at `compare/vx.y.z...HEAD`.
3. Land those changelog edits on `main` through a pull request (see the
   contribution workflow above), then tag and push:

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

Please open a GitHub issue before starting large changes or proposing new features, so scope
and approach can be settled before code is written. Bug reports should include:
- macOS version
- `container --version` output
- Steps to reproduce
- What you expected vs what happened
