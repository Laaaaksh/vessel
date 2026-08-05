#!/usr/bin/env python3
"""File top-class TUI child issues under epic #11 (or create epic if missing)."""
from __future__ import annotations

import json
import os
import subprocess
import sys

REPO = "Laaaaksh/vessel"
os.environ.setdefault("GH_CONFIG_DIR", os.path.expanduser("~/.config/gh-personal"))


def run(args: list[str], input_text: str | None = None) -> str:
    p = subprocess.run(
        args,
        input=input_text,
        text=True,
        capture_output=True,
        check=False,
    )
    if p.returncode != 0:
        sys.stderr.write(p.stderr)
        raise SystemExit(f"command failed: {' '.join(args)}")
    return p.stdout.strip()


def issue_create(title: str, labels: str, body: str) -> str:
    return run(
        [
            "gh",
            "issue",
            "create",
            "--repo",
            REPO,
            "--title",
            title,
            "--label",
            labels,
            "--body",
            body,
        ]
    )


def ensure_epic() -> tuple[int, str]:
    raw = run(
        [
            "gh",
            "issue",
            "list",
            "--repo",
            REPO,
            "--state",
            "open",
            "--json",
            "number,title",
            "--limit",
            "50",
        ]
    )
    for item in json.loads(raw):
        if item["title"].startswith("Epic: top-class Vessel TUI"):
            url = f"https://github.com/{REPO}/issues/{item['number']}"
            return item["number"], url
    url = issue_create(
        "Epic: top-class Vessel TUI (lazygit-feel)",
        "epic,enhancement,area/ui",
        """## Goal
Make Vessel a **lazygit-class** Apple Container TUI: focused panels, obvious selection, context-aware help, safe destructive actions, and a shell/logs loop that never leaves the UI looking broken.

## Plan
Local: `~/Documents/Work/Planning/tech-plans/2026-08-06-vessel-top-class-tui.md`

## Definition of done (v1 = must slices)
1. Enter shell, mess around, exit - UI is pristine.
2. Arrow through mixed running/stopped list - selection obvious every row.
3. New user presses `?` and can operate without the README.
4. Footer never lies about what the next key will do.
5. No config flag advertises a feature the code does not implement.

## Explicitly out of scope
Docker/Podman/k8s backends, compose, dive clone, web UI, non-darwin targets.
""",
    )
    return int(url.rsplit("/", 1)[-1]), url


def main() -> None:
    epic_num, epic_url = ensure_epic()
    print(f"EPIC #{epic_num} — {epic_url}")

    children: list[tuple[str, str, str]] = [
        (
            "fix: shell ExecProcess restores alt-screen cleanly",
            "bug,priority/must,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **1** (must, first)

## Problem
Entering a container shell redirects to the terminal; on exit Vessel UI is spoiled (alt-screen / cursor / stale frame). Known bubbletea ExecProcess handoff class of bugs.

## Done when
Enter shell → exit → UI looks identical to pre-shell. Poller/tick paused during exec.

## Approach
- `modeShell` with empty `View()` while execing
- Pause tick/poller
- On `shellDoneMsg`: clear screen + full refresh / re-enter alt-screen
- Consider wrapping clear + `container exec`
- Prefer `tea.WithAltScreen()` at program start (`main.go`)

## Touches
`main.go`, `internal/ui/app.go`, maybe `internal/backend/containers.go`

## Verified by
Live repro + fake-exec integration test.

## Plan
`2026-08-06-vessel-top-class-tui.md` section slice 1 / capability A1
""",
        ),
        (
            "fix: selection highlight uses a single style pass",
            "bug,priority/must,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **2** (must, parallel with slice 1)

## Problem
Navigating containers: highlights are wrong. Selected row wraps already-colored cells; status greens fight purple selection. Root `styles.tableRowSelected` is unused; panels duplicate hex.

## Done when
Selected row is one solid highlight. Status colour only when unselected. Same pattern on images/volumes.

## Touches
`internal/ui/styles.go`, `internal/ui/containers/view.go`, images/volumes views

## Verified by
Golden View tests at fixed 120x40; live eyeball running/exited rows.
""",
        ),
        (
            "feat: pane focus model + honest mouse",
            "enhancement,priority/must,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **3** (must; after 1-2)

## Problem
No list vs detail vs sidebar focus. Mouse mode is enabled in config but Update never handles mouse.

## Done when
- Explicit focus: sidebar | list | detail
- `h`/`l` or arrows move focus; j/k only move focused list
- Focused pane visually brighter
- Mouse: click-to-select + wheel **or** default `mouse_enabled=false` until handlers exist

## Touches
`internal/ui/app.go`, panel Updates, `internal/config/config.go`, `main.go`, README

## Verified by
UI Update tests; README matches config defaults.
""",
        ),
        (
            "feat: context footer, contextual help, KeyMap router, confirm modal",
            "enhancement,priority/must,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **4** (must)

## Done when
- Footer always shows view, focus, `n/N`, and 4-6 keys that work *now*
- `?` is panel/focus-aware; disabled actions say why
- All matching goes through `keys.go`
- Delete uses a real centered confirm modal
- Sticky error vs ephemeral toast; action spinner

## Touches
`internal/ui/keys.go`, `app.go`, new `internal/ui/chrome/`

## Decision in this slice
Adopt Charm `bubbles` vs keep custom panels - pick one and stick.

## Verified by
Update tests for help/confirm; visual goldens.
""",
        ),
        (
            "feat: filter on images/volumes + page scroll",
            "enhancement,priority/must,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **5** (must)

## Done when
- `/` filter on containers, images, and volumes
- Match count shown; esc clears
- PgUp/PgDn or ctrl+d/u page scroll in lists and logs

## Touches
Panel models + shared filter/textinput helper

## Verified by
Model tests; fake fleet with >1 page of items.
""",
        ),
        (
            "chore: UI Update + golden View test harness",
            "enhancement,priority/must,area/dx,area/ui",
            f"""## Parent
Epic #{epic_num} — ongoing (start with slices 1-2)

## Done when
- Table-driven Update tests for keys/modes
- Golden View frames under fixed width
- Fake ExecProcess resume test
- Extend `scripts/smoke.sh` for shell round-trip when possible

## Why must
Without this, shell/selection fixes regress silently.
""",
        ),
        (
            "spike: Apple container 1.2 subcommand matrix (prune/networks/create/events)",
            "enhancement,priority/should,area/backend",
            f"""## Parent
Epic #{epic_num} — unlocks honest demotion for images/volumes/system

## Deliverable
List which exist on local Apple container 1.2.x with exact flags:
- image pull progress / prune / history
- volume create / prune / inspect users
- system status / disk
- networks
- events
- container create/run variants

## Done when
Each capability tagged **supported** / **unsupported**.

## Blocks
Slices 8, 9, 13, and any system/networks views.
""",
        ),
        (
            "feat: container inspect depth + yank/copy",
            "enhancement,priority/should,area/ui,area/backend",
            f"""## Parent
Epic #{epic_num} — slice **6** (should)

## Done when
- Detail panel uses `InspectContainer` (labels, mounts, image id, ...)
- `y` copies selected id/name (macOS `pbcopy`)
- Shell chooser: prefer bash if present else sh; config override

## Touches
`backend/containers.go`, containers detail, clipboard helper, config

## Verified by
Fake inspect fixture; live alpine container.
""",
        ),
        (
            "feat: logs search, follow toggle, yank line",
            "enhancement,priority/should,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **7** (should)

## Done when
- `f` freeze/follow with indicator
- `/` search in buffer; `n`/`N` next/prev
- `y` yank line; wrap/raw toggles

## Touches
`internal/ui/logs/`, `app.go` stream wiring

## Verified by
Log model tests; live follow freeze/resume.
""",
        ),
        (
            "feat: image pull (+ prune if Apple CLI supports)",
            "enhancement,priority/should,area/ui,area/backend",
            f"""## Parent
Epic #{epic_num} — slice **8** (should; spike first)

## Spike
Document Apple container 1.2.x image subcommands: pull progress, prune, history. Demote anything missing.

## Done when
- `p` pulls with visible progress/errors
- Prune only if CLI supports
- Filter/sort on images list

## Touches
`backend/images.go`, images panel

## Verified by
Fake pull stream; live pull of a small image.
""",
        ),
        (
            "feat: volumes create/prune/users (honest demotion if unsupported)",
            "enhancement,priority/should,area/ui,area/backend",
            f"""## Parent
Epic #{epic_num} — slice **9** (should; spike first)

## Spike
Which volume create/prune/inspect-users commands exist on Apple container 1.2.x?

## Done when
- Filter/sort/path copy parity
- Create/prune if supported; else honest unsupported
- Show which containers use a volume when possible

## Touches
`backend/volumes.go`, volumes panel

## Verified by
Fake CLI; live volume create/rm when supported.
""",
        ),
        (
            "feat: action menu + custom commands",
            "enhancement,priority/should,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **10** (should)

## Done when
- `x` opens action menu for current selection
- Custom commands in TOML with templates (`{{.Name}}`, `{{.ID}}`)
- Example in `config.example.toml`

## Touches
Menu component; `config.go`; README

## Verified by
Template unit tests; live custom inspect.
""",
        ),
        (
            "feat: metrics history + fleet overview",
            "enhancement,priority/should,area/ui,area/backend",
            f"""## Parent
Epic #{epic_num} — slice **11** (should)

## Done when
- Last N CPU/mem samples per container; colour thresholds
- Optional fleet header stats / top offenders
- History must not block UI

## Touches
`backend/metrics.go`, containers view

## Verified by
Poller unit tests with synthetic samples.
""",
        ),
        (
            "feat: layout modes (half/full) + command log strip",
            "enhancement,priority/later,area/ui",
            f"""## Parent
Epic #{epic_num} — slice **12** (later)

## Done when
- `+`/`_` cycle normal → logs-emphasis → full list
- Toggleable command log strip (last N `container ...` invocations)

## Touches
`internal/ui/app.go`

## Gate
Do not start until must slices 1-5 feel excellent.
""",
        ),
        (
            "feat: run/create wizards + multi-select bulk ops",
            "enhancement,priority/later,area/ui,area/backend",
            f"""## Parent
Epic #{epic_num} — slice **13** (later)

## Done when
- Create/run container from image (wizard) if Apple CLI supports cleanly
- Multi-select; bulk stop/rm with confirm summary

## Gate
After must 1-5; depends on CLI spike outcomes from slices 8-9.
""",
        ),
        (
            "feat: theme + key remaps + vessel doctor + README tour",
            "enhancement,priority/later,area/ui,area/dx,documentation",
            f"""## Parent
Epic #{epic_num} — slice **14** (later; doctor can pull earlier)

## Done when
- Theme tokens in config
- Keybind remaps in TOML
- `vessel doctor`
- `config.example.toml`; README key tour / gif

## Verified by
Doctor on this machine; config round-trip test.
""",
        ),
    ]

    urls: list[str] = []
    for title, labels, body in children:
        url = issue_create(title, labels, body)
        print(url)
        urls.append(f"- {url} — {title}")

    must = "\n".join(urls[:6])
    should = "\n".join(urls[6:13])
    later = "\n".join(urls[13:])
    comment = f"""## Child issues (build order)

### Must
{must}

### Should
{should}

### Later
{later}

Plan: `~/Documents/Work/Planning/tech-plans/2026-08-06-vessel-top-class-tui.md`
"""
    run(["gh", "issue", "comment", str(epic_num), "--repo", REPO, "--body", comment])
    print("DONE")


if __name__ == "__main__":
    main()
