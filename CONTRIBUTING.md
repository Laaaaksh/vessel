# Contributing to vessel

Thank you for your interest in contributing. vessel is a keyboard-driven terminal UI for
Apple's native Mac containers, open source under the MIT license.

## Contributor License Agreement (CLA)

Before your first pull request is merged, you must sign the Contributor License Agreement.
The CLA bot will comment on your PR with instructions when you open one.

The CLA grants the project maintainer the right to use your contribution under any license,
including for future commercialization. This is standard practice for open-source projects
that may be acquired or relicensed.

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
5. Push the branch to your fork.
6. Open a pull request against `main` here. The CLA bot will prompt you to sign the CLA
   if you haven't already.

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
