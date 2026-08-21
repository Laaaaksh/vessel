# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

- Add durable project-specific notes here as they are discovered through real work.

## UI key handling

- Bubble Tea `KeyPressMsg.String()` serialises a space bar press as `"space"`, never a literal `" "`. A binding holding `" "` compiles and silently never fires. `KeyMap.ToggleMark` (`internal/ui/keys.go`) is therefore `"space"`, and `Model.withKeys` in `internal/ui/app.go` pushes it into all three panels via `SetToggleMarkKey`, which match that value rather than a literal of their own. The same serialisation makes free-text input wrong if it gates characters on `len(k) == 1`: that is a *byte* length, so it silently drops both the literal space (which never arrives; it arrives as `"space"`) and any multi-byte rune (accents, CJK, emoji). `handlePromptKey` and `runForm.insert` (`internal/ui/app.go`, `internal/ui/runform.go`) match `"space"` explicitly and gate on `utf8.RuneCountInString` instead. The three panel filter inputs (`internal/ui/{containers,images,volumes}/model.go`) still gate on byte length and were deliberately left unfixed as out of scope for the run-form work - filtering by a name with a space or an accent will drop those characters.
- Multi-select ("space to mark, `d` to bulk delete") is implemented identically in all three panes: `marked map[string]bool` and `MarkedIDs()`, which iterates only `filtered` so marks never surface for items dropped by a refresh/filter. See `internal/ui/containers/model.go` as the canonical copy. Every delete funnels through `beginDelete`/`confirmDelete` in `internal/ui/app.go`, which stage targets as a `deleteKind` plus `pendingIDs []string`. Mark lifetime is owned solely by `SetItems`, which drops marks whose identity is absent from the incoming list: marks key off identities that outlive a removal (an image digest, a volume name), and pruning at the one place every removal path (confirm, prune, an external `container delete`) refreshes through is what stops them resurfacing on recreate. A delete that fails therefore keeps its marks and stays retryable, and marks the delete did not touch survive.
- With exactly one mark, `d` deletes the cursor row, not the marked item - deliberate parity across all three panes, not a bug. Changing it is a known limitation, not yet filed.
- Image marks key on digest+reference, not the digest alone (`markKey` in `internal/ui/images/model.go`): two tags of one digest are two rows, and `MarkedIDs()` still emits each digest once because the delete takes digests.
- Panel rows carry a 1-char mark cell absorbed into the first column (`mark + Pad(value, col-1)`), so rows stay aligned with an unchanged header. Column widths are named constants per panel; `TestRenderRowAlignsWithHeader` guards the invariant in each package.
- `backend.Client.RemoveImage`/`RemoveVolume` take variadic ids/names and refuse an empty call (`errNoDeleteTargets` in `internal/backend/client.go`). A bare `container image delete` destroys nothing - the CLI needs `--all` for that - so the guard is there to catch a caller bug that would otherwise surface as a confusing CLI usage error.
- Every CLI invocation is re-wrapped with `Client.timeout` (10s, `internal/backend/client.go`), so the context a caller passes never widens it. A bulk image or volume delete batches every id into one invocation and shares that single 10s budget, so a large one can be killed partway and reported as a failure with nothing actually wrong; bulk container deletes issue one call per id and so get 10s each.
- A modal built from `Border` + `Padding` must size its inner content from that style's own `lipgloss.Style.GetFrameSize()`, not a guessed constant: guessing (e.g. `width-4`) is exactly what silently overflowed the run form past a 60x12 terminal during development. See `runFormModal` in `internal/ui/runform.go` for the pattern.
- `container run`/`container exec` support (`internal/backend/run.go`) is a deliberate flag subset - `-p -e -v -m -c -d -t -i --name --arch` for run, plus a one-shot `Exec` that shells a single command through the container's own shell (`sh -c`, not an interactive attach). `Client.Exec` is distinct from `ShellCmd`: `ShellCmd` returns an `*exec.Cmd` for `tea.ExecProcess` to attach a TTY to (`internal/ui/app.go`'s `openShell`); `Exec` blocks and returns captured output for a single command (`internal/ui/app.go`'s `beginExec`/`submitExec`, bound to `e`).

## Build / test / lint
- `make test` and `make lint` before anything else; [CONTRIBUTING.md](CONTRIBUTING.md#development-workflow) owns the workflow, including the live Apple-CLI test variants and `scripts/smoke.sh`.
- `golangci-lint` must be v2.12.2+: an older build refuses to load `.golangci.yml` when `go.mod` targets a newer Go than the binary was built with. The goimports group uses `github.com/Laaaaksh/vessel` as the local prefix.
- `main` is lint-clean; keep it that way and fix any issue a diff introduces.

## Live CLI probes
- Live TUI/backend probes need the Apple `container` CLI installed with its system services running (`container system status`); `container list --all --format json` reads live state.
- Destroy nothing shared: a probe machine's containers, images and volumes may belong to someone else. Exercise prune and other destructive verbs through the confirm modal's cancel path, or against throwaway resources cleaned up immediately.
- To reproduce the "services down" string-matched hint while services are up, shadow PATH with a wrapper returning the CLI's documented error for one verb and delegating the rest, then run the real binary in a PTY.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
