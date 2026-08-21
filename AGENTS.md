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

## Detail-pane row budgets

- `uiutil.Pane.Add` charges each row against a fixed rendered-row budget and drops everything from the first row that doesn't fit onward (`internal/ui/uiutil/layout.go`); `uiutil.KV` gives a value no width bound, so a value long enough to wrap (a version string, a long path) can consume the whole remaining budget and silently drop every row after it, not just wrap ugly. Any value whose length isn't already bounded by the data (contrast a short "local"/"ext4" driver/format) must go through `KVFit`, not `KV`. Reproduce with the real pane width, not the terminal width: `internal/ui/app.go`'s `layoutDims` shrinks a 60-wide terminal's detail pane down to ~18 columns, and a value string short enough to fit a 60-wide test render can still overflow that.
- `container system status --format json` exits 1 while still printing a valid, parseable JSON body (`status: "unregistered"`) when the services have never been started - `backend.Client.runRaw` exists so that body isn't discarded the way `run`/`runJSON` discard stdout on any non-zero exit. `container system df --format json` has no such fallback: it prints a plain-text error on the same down state, so a services-down `DiskUsage` call is a real error for the caller to handle, even though the sibling `SystemStatus` call for the same state is not. See `internal/backend/system.go`.

## Container CLI sharp edges

- Vessel deliberately does NOT own registry login. A refused `image push` splits
  two ways in `internal/backend/images.go`, and the advice is deliberately
  opposite: `credentialStderrPhrases` (401 and friends) tells the user to run
  `container registry login`; `permissionStderrPhrases` (403) tells them login
  will NOT help, because the session is valid and the account simply lacks write
  access. Do not fold 403 back into the credential list or name the login command
  in its message — a 403 does not establish that the credentials were rejected.
- Classify a CLI failure from `CLIError.Stderr` (`internal/backend/client.go`),
  never from `err.Error()` — but stderr echoes the image reference too, so match
  multi-word phrases only a registry emits ("401 unauthorized", "no credentials
  found"). A bare "unauthorized" misreads `myorg/unauthorized-proxy:v1`.
- The footer flattens and truncates the error/status lines it renders (`footerLine`
  in `internal/ui/app.go`, via `uiutil.TruncateCells`): CLI errors carry raw
  multi-line stderr and must be flattened to one row. The key-hint branch is
  deliberately exempt — its grouping is authored to be read as-is. So never
  route unbounded text through the footer expecting it to be readable; the
  images detail pane is the surface for anything longer (see its notice, which
  is charged against the pane's row budget on top of, not instead of, the
  normal content so it is never itself the thing that gets dropped).
- On the installed 1.2.2 build (services running) `image save/load/tag/push` are
  core subcommands and `image pull` works live; honour the plugin gate only when
  a probe says so. `docs/APPLE_CONTAINER_MATRIX.md` records earlier probe results.
- `image tag <source> <target>` and `image save --output <path> <ref>` argument
  order is asserted in tests via `Client.CommandLog`; don't swap the order.
- `Client.run` caps EVERY invocation at `defaultTimeout` (10s, see
  `internal/backend/client.go`), which silently overrides the longer budget the
  UI passes in. So `image save`/`load`/`push`/`pull` of a large image is killed
  mid-transfer and reports a context deadline, not a real failure. This ships
  known-broken for large images. The shared-timeout fix is a known limitation,
  not yet filed, and is deliberately out of the image-mobility scope. The images
  help view states the same caveat to the user.

## Release pipeline

- `.goreleaser.yml` still uses `brews:`, not the newer `homebrew_casks:`, despite goreleaser's
  deprecation warning. Confirmed via `goreleaser check` (installed v2.17.1) and goreleaser's own
  deprecation notice: `homebrew_casks` installs via Homebrew Cask semantics, not a Formula - it
  needs `brew install --cask` once any name collision exists, and even unambiguous installs go
  through a Cask `binary` shim rather than a formula's direct `bin.install`. That's a real
  `brew install` UX change for a plain CLI, so migrating now is deliberately deferred. When `brews:`
  is finally removed upstream, re-evaluate against the then-current `homebrew_casks:` docs before
  switching.
- `version`/`commit`/`date` in `main.go` must stay package-level `var`s, not `const`s - the
  `-X main.version=...` linker flag goreleaser passes silently no-ops against a `const`.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
