# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

- Add durable project-specific notes here as they are discovered through real work.

## UI key handling

- Bubble Tea `KeyPressMsg.String()` serialises a space bar press as `"space"`, never a literal `" "`. A binding holding `" "` compiles and silently never fires. `KeyMap.ToggleMark` (`internal/ui/keys.go`) is therefore `"space"`, and `Model.withKeys` in `internal/ui/app.go` pushes it into all three panels via `SetToggleMarkKey`, which match that value rather than a literal of their own.
- Multi-select ("space to mark, `d` to bulk delete") is implemented identically in all three panes: `marked map[string]bool` and `MarkedIDs()`, which iterates only `filtered` so marks never surface for items dropped by a refresh/filter. See `internal/ui/containers/model.go` as the canonical copy. Every delete funnels through `beginDelete`/`confirmDelete` in `internal/ui/app.go`, which stage targets as a `deleteKind` plus `pendingIDs []string`. Mark lifetime is owned solely by `SetItems`, which drops marks whose identity is absent from the incoming list: marks key off identities that outlive a removal (an image digest, a volume name), and pruning at the one place every removal path (confirm, prune, an external `container delete`) refreshes through is what stops them resurfacing on recreate. A delete that fails therefore keeps its marks and stays retryable, and marks the delete did not touch survive.
- With exactly one mark, `d` deletes the cursor row, not the marked item - deliberate parity across all three panes, not a bug. Changing it is tracked separately as `vessel-mark-wins-over-cursor`.
- Image marks key on digest+reference, not the digest alone (`markKey` in `internal/ui/images/model.go`): two tags of one digest are two rows, and `MarkedIDs()` still emits each digest once because the delete takes digests.
- Panel rows carry a 1-char mark cell absorbed into the first column (`mark + Pad(value, col-1)`), so rows stay aligned with an unchanged header. Column widths are named constants per panel; `TestRenderRowAlignsWithHeader` guards the invariant in each package.
- `backend.Client.RemoveImage`/`RemoveVolume` take variadic ids/names and refuse an empty call (`errNoDeleteTargets` in `internal/backend/client.go`). A bare `container image delete` destroys nothing - the CLI needs `--all` for that - so the guard is there to catch a caller bug that would otherwise surface as a confusing CLI usage error.
- Every CLI invocation is re-wrapped with `Client.timeout` (10s, `internal/backend/client.go`), so the context a caller passes never widens it. A bulk image or volume delete batches every id into one invocation and shares that single 10s budget, so a large one can be killed partway and reported as a failure with nothing actually wrong; bulk container deletes issue one call per id and so get 10s each.
- The sidebar `View` enum (`internal/ui/types.go`) has a matching `numViews` constant; Tab/j/k cycling in `internal/ui/app.go` uses `% numViews`, not a hardcoded modulo — adding a view means bumping that constant, not counting cases by hand.
- `internal/ui/networks/` is deliberately list-and-inspect only (no create/delete/prune) — the network view was scoped that way on purpose, to confirm the live `container network` JSON shape before any destructive action is built on it. It has no mark/multi-select support for the same reason (nothing to bulk-delete). Its tests, and `internal/backend/networks_test.go`, use `testify/suite` with camelCase scenario methods and named (non-table-driven) tests; every other package in this repo still uses plain `testing.T` — that split is intentional per the task that introduced networks, not drift to reconcile in either direction without asking.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
