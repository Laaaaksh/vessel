# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

- Add durable project-specific notes here as they are discovered through real work.

## UI key handling

- Bubble Tea `KeyPressMsg.String()` serialises a space bar press as `"space"`, never a literal `" "`. Match on `"space"` or the mark/bulk-toggle key never fires. All three panel models match this in `internal/ui/{containers,images,volumes}/model.go`.
- Multi-select ("space to mark, `d` to bulk delete") is implemented identically in all three panes: `marked map[string]bool`, `MarkedIDs()` iterates only `filtered` (so marks never surface for items dropped by a refresh/filter), `ClearMarks()`. See `internal/ui/containers/model.go` as the canonical copy. Every delete funnels through `beginDelete`/`confirmDelete` in `internal/ui/app.go`, which stage targets as a `deleteKind` plus `pendingIDs []string`; `confirmDelete` is also the one place that calls `ClearMarks()`, because marks key off identities that outlive a delete (an image digest, a volume name) and would otherwise resurface on recreate.
- With exactly one mark, `d` deletes the cursor row, not the marked item - deliberate parity across all three panes, not a bug. Changing it is tracked separately as `vessel-mark-wins-over-cursor`.
- Panel rows carry a 1-char mark cell absorbed into the first column (`mark + Pad(value, col-1)`), so rows stay aligned with an unchanged header. Column widths are named constants per panel; `TestRenderRowAlignsWithHeader` guards the invariant in each package.
- `backend.Client.RemoveImage`/`RemoveVolume` take variadic ids/names and refuse an empty call (`errNoDeleteTargets` in `internal/backend/client.go`) because a bare `container image delete` deletes everything.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
