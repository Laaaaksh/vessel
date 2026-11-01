# Security Policy

## Supported versions

vessel is a young project. Security fixes are made against the **latest
release** and `main` only — please confirm you can reproduce the issue on the
newest release (`vessel --version`) before reporting.

| Version          | Supported |
| ---------------- | --------- |
| latest release   | yes       |
| older releases   | no        |

## Reporting a vulnerability

Please do **not** open a public GitHub issue for anything you believe is a
security problem.

Use GitHub's private vulnerability reporting instead:

> https://github.com/Laaaaksh/vessel/security/advisories/new

That link reaches the maintainer privately — the report, follow-up discussion,
and any fix coordination stay confidential until a patched release ships.

When reporting, please include:

- your `vessel --version` output
- your macOS version and Apple Silicon chip
- your `container --version` output
- clear steps to reproduce, including any config (`config.toml`) involved

## What belongs in a report

vessel is a local terminal dashboard that drives the Apple `container` CLI.
Things worth reporting:

- Any path where untrusted data — an image or registry name, container/log
  output rendered in the UI — causes command execution outside the intended
  `container` CLI invocation.
- vessel reading from or writing to files outside its documented locations
  (`~/.config/vessel/config.toml` and similar state paths).
- The [[custom_commands]] feature executing anything the user did not
  explicitly configure themselves.

Out of scope:

- Bugs in the Apple `container` runtime itself — please report those to Apple
  via its own channels.
- Issues that require an attacker to already run arbitrary commands as your
  user: vessel intentionally shells out to the `container` binary and executes
  the custom commands found in your local config, so local code execution is
  assumed trusted input by design.

## Credits

Reporters who wish to be credited in a fix's release notes may say so in the
private report; otherwise reports are handled without attribution.
