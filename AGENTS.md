# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

- Add durable project-specific notes here as they are discovered through real work.

## UI key handling

- Resting footer hints are derived, never literal: `footerHints`/`footerViewHints` (`internal/ui/keys.go`) build each view's line from `KeyMap` fields and resolve shadowing through `activeCustomCommands`, the same first-wins pass help's `withCustomBindings` uses. A key an active custom command claims stops advertising its built-in; multi-key groups split in place (`[s] custom: x  [u/r] lifecycle`), and the System view's `[j/k]` group can never split (nav aliases are reserved). Do not reintroduce hardcoded hint strings — the byte-for-byte default renderings are pinned per-view in `internal/ui/app_footer_test.go`. The "custom: name" display convention lives in `customHintLabel`, shared by help rows, the action menu and the footer.
- Custom commands bound to a key they can never fire on (reserved, unproducible spelling, or a key an earlier entry already claimed) or carrying no command are silently dropped from dispatch AND help by `customKey`/`customCommandFor` (`internal/ui/keys.go`); `classifyCustomKey` in the same file resolves that exact per-entry predicate chain plus a reason, `unusableBindings` layers the list-level duplicate check on top in the same first-wins order dispatch uses, and the startup notice (`ignoredKeysNotice`, surfaced through `setStatus` in `newModel`) reports all of them once via the ordinary footer lifecycle. A failed config load joins that same notice in `New()` (`configLoadNotice` + `joinNotices`, app.go): error text first, config path trailing, because the footer truncates from the tail and the head must survive. `config.Load` also sanitizes on EVERY decode path (`sanitize`, internal/config/config.go): a failed decode leaves `PollInterval` zeroed because BurntSushi's UnmarshalText stores ParseDuration's zero result before returning its error, and `Poller.Run` panics in `time.NewTicker` on any non-positive interval — including a successfully parsed `"0s"` — so the clamp must never be narrowed to the success path only. Which user values survive a broken file is nondeterministic (keys apply in map-iteration order), so tests may only assert error-plus-safe-fields, never which authored values survived. `KeyMap.Reserved` must list every key `handleKey` answers before it reaches `customCommandForKey` — the numeric view shortcuts are `1`-`5`, so all five are reserved. The notice's "still in the action menu" tail hangs off the individual reason, not the whole message: an entry with no command has nothing to run there either, so it must never carry that tail. A missing key with a command present is the documented action-menu-only form and is deliberately never reported. Because `New()` can now set a startup status, ui-package tests must build models via `newModel(cfg)` / `newTestModel()`, not bare `New()`, when they assert footer or status state — bare `New()` reflects the host's dotfiles.
- Custom commands execute through `runCustom` (`internal/ui/app.go`) as `bash -lc` with a 60s budget, and stdout/stderr are captured SEPARATELY on purpose: the login shell sources `~/.profile`, whose chatter lands on stderr, so a merged capture made that noise part of the result text and pushed the command's real output past the footer's 80-char truncation. Success displays trimmed stdout only (empty → "custom ok", deliberately no stderr fallback — the fallback re-showed profile noise for every silent command); failure wraps the exit status plus trimmed stderr (stdout as fallback diagnostics). `TestRunCustom_*` in app_test.go pins all three branches via a temp-HOME chatty `.bash_profile`; keep new shell-outs that display output on split capture.
- Bubble Tea `KeyPressMsg.String()` serialises a space bar press as `"space"`, never a literal `" "`. A binding holding `" "` compiles and silently never fires. `KeyMap.ToggleMark` (`internal/ui/keys.go`) is therefore `"space"`, and `Model.withKeys` in `internal/ui/app.go` pushes it into all three panels via `SetToggleMarkKey`, which match that value rather than a literal of their own. The same serialisation makes free-text input wrong if it gates characters on `len(k) == 1`: that is a *byte* length, so it silently drops both the literal space (which never arrives; it arrives as `"space"`) and any multi-byte rune (accents, CJK, emoji). Every free-text surface — `handlePromptKey` and `runForm.insert` (`internal/ui/app.go`, `internal/ui/runform.go`) plus the five panel filter/search inputs (`internal/ui/{containers,images,volumes,networks,logs}/model.go`) — matches `"space"` explicitly, gates on `utf8.RuneCountInString(k) == 1`, and trims backspace with `utf8.DecodeLastRuneInString`; keep any new text-entry surface on that same pattern or it will drop spaces and accents again.
- Multi-select ("space to mark, `d` to bulk delete") is implemented identically in all three panes: `marked map[string]bool` and `MarkedIDs()`, which iterates only `filtered` so marks never surface for items dropped by a refresh/filter. See `internal/ui/containers/model.go` as the canonical copy. Every delete funnels through `beginDelete`/`confirmDelete` in `internal/ui/app.go`, which stage targets as a `deleteKind` plus `pendingIDs []string`. Mark lifetime is owned solely by `SetItems`, which drops marks whose identity is absent from the incoming list: marks key off identities that outlive a removal (an image digest, a volume name), and pruning at the one place every removal path (confirm, prune, an external `container delete`) refreshes through is what stops them resurfacing on recreate. A delete that fails therefore keeps its marks and stays retryable, and marks the delete did not touch survive.
- With exactly one visible mark, `d` targets the marked row and the cursor is ignored - marking is the explicit intent signal (`SingleMarked()` on each panel, consumed by the `beginDelete*` funcs in `internal/ui/app.go`). Zero marks falls back to the cursor row; multiple marks bulk-delete as before. The split is identical in all three panes.
- Image marks key on digest+reference, not the digest alone (`markKey` in `internal/ui/images/model.go`): two tags of one digest are two rows, so marking both counts as several marks for `SingleMarked()` even though `MarkedIDs()` emits each digest once because the delete takes digests.
- Panel rows carry a 1-char mark cell absorbed into the first column (`mark + Pad(value, col-1)`), so rows stay aligned with an unchanged header. Column widths are named constants per panel; `TestRenderRowAlignsWithHeader` guards the invariant in each package.
- `backend.Client.RemoveImage`/`RemoveVolume` take variadic ids/names and refuse an empty call (`errNoDeleteTargets` in `internal/backend/client.go`). A bare `container image delete` destroys nothing - the CLI needs `--all` for that - so the guard is there to catch a caller bug that would otherwise surface as a confusing CLI usage error.
- CLI invocation budgets live at the shared run boundary (`internal/backend/client.go`): quick commands get `defaultTimeout` (10s) - the client's own quick-command budget with no UI counterpart - while every verb known to run long passes an explicit override via `runWithTimeout`: start/stop/restart get `lifecycleTimeout` (30s), image pull/tag/save/load/push, all three prunes and `Run` (`container run` pulls a missing image and boots a VM) share `globalTimeout` (120s), batched `RemoveImage`/`RemoveVolume` calls get `confirmTimeout` (60s) for the whole call, and the one-shot `Exec` gets `execTimeout` (30s) because its command is user-authored. Those four mirror identically sized outer bounds in `internal/ui/app.go`; the pairing is pinned on both sides — `TestLongOperationBudgets_matchInternalUIOuterBounds` in `internal/backend` and `TestOuterBounds_matchBackendPerCallBudgets` in `internal/ui` — so change both halves together or expect a red test. Full call-site audit, recorded here so no verb-by-verb rounds remain: everything still on the quick default is deliberate - list/inspect/status reads (`List*`, `Inspect*`, `SystemStatus`, `DiskUsage`), `TailLogs`, `CreateVolume`, and single-target `RemoveContainer` (bulk container deletes still issue one call per id, and a single delete is quick).
- The sidebar `View` enum (`internal/ui/types.go`) has a matching `viewCount` constant; Tab/j/k cycling in `internal/ui/app.go` uses `% viewCount`, not a hardcoded modulo — adding a view means bumping that constant, not counting cases by hand. Numeric shortcuts `1`-`5` and Tab/sidebar-nav order both follow enum order: Containers, Images, Volumes, System, Networks.
- `internal/ui/networks/` is deliberately list-and-inspect only (no create/delete/prune) — the network view was scoped that way on purpose, to confirm the live `container network` JSON shape before any destructive action is built on it. It has no mark/multi-select support for the same reason (nothing to bulk-delete). Its tests, and `internal/backend/networks_test.go`, use `testify/suite` with camelCase scenario methods and named (non-table-driven) tests; every other package in this repo still uses plain `testing.T` — that split is intentional per the task that introduced networks, not drift to reconcile in either direction without asking.
- A modal built from `Border` + `Padding` must size its inner content from that style's own `lipgloss.Style.GetFrameSize()`, not a guessed constant: guessing (e.g. `width-4`) is exactly what silently overflowed the run form past a 60x12 terminal during development. See `runFormModal` in `internal/ui/runform.go` for the pattern.
- `container run`/`container exec` support (`internal/backend/run.go`) is a deliberate flag subset - `-p -e -v -m -c -d -t -i --name --arch` for run, plus a one-shot `Exec` that shells a single command through the container's own shell (`sh -c`, not an interactive attach). `Client.Exec` is distinct from `ShellCmd`: `ShellCmd` returns an `*exec.Cmd` for `tea.ExecProcess` to attach a TTY to (`internal/ui/app.go`'s `openShell`); `Exec` blocks and returns captured output for a single command (`internal/ui/app.go`'s `beginExec`/`submitExec`, bound to `e`). An attached (non-`-d`) run blocks the action status until the 120s budget expires, because the CLI prints nothing until the container exits; streaming output or detached-launch support for long-running foreground sessions is future work.
- README's keybindings table, `helpBindings` (`internal/ui/keys.go`) and `KeyMap` are three surfaces for one truth; README↔help parity is guarded by `internal/ui/readme_parity_test.go`, which parses the table (arrows, `` `` ` `` `` quoting and `1`-`5` ranges normalized) and fails both on a README key help omits in its scoped views and on a help-advertised key no README row names. Adding or renaming a binding means touching all three plus a scope entry in the test; an unmapped new README row fails loudly rather than passing silently.

## Build / test / lint
- `make test` and `make lint` before anything else; [CONTRIBUTING.md](CONTRIBUTING.md#development-workflow) owns the workflow, including the live Apple-CLI test variants and `scripts/smoke.sh`.
- `golangci-lint` must be v2.12.2+: an older build refuses to load `.golangci.yml` when `go.mod` targets a newer Go than the binary was built with. The goimports group uses `github.com/Laaaaksh/vessel` as the local prefix.
- `main` is lint-clean; keep it that way and fix any issue a diff introduces.

## Live CLI probes
- Live TUI/backend probes need the Apple `container` CLI installed with its system services running (`container system status`); `container list --all --format json` reads live state.
- Destroy nothing shared: a probe machine's containers, images and volumes may belong to someone else. Exercise prune and other destructive verbs through the confirm modal's cancel path, or against throwaway resources cleaned up immediately.
- To reproduce the "services down" string-matched hint while services are up, shadow PATH with a wrapper returning the CLI's documented error for one verb and delegating the rest, then run the real binary in a PTY.

## Footer error durability

- `Model.errDurable` (`internal/ui/app.go`) marks an action failure - set via `setActionErr` from the `actionDoneMsg` case - as surviving both the containers refresh that verb's own handler triggers and any later periodic tick's refresh: `applyContainersLoaded`'s self-heal-on-success only clears `lastErr` when `!errDurable` (alongside the pre-existing services-down carve-out). It clears the normal way - every `setLastErr` call resets the flag, so a later action's success or a fresh error from any other path supersedes it. Everything else stays non-durable, deliberately and for different reasons. Background load errors (`containersLoadedMsg`/`imagesLoadedMsg`/`volumesLoadedMsg`/`networksLoadedMsg`) are self-correcting: a later success of that *same* call is real evidence the problem resolved. `shellDoneMsg`, `logsOpenedMsg` and the two clipboard-yank failures are user-initiated with no such signal and so are still wiped by the next successful poll, exactly like the `actionDoneMsg` bug `errDurable` fixes - a scope decision, not evidence durability is unwanted there. An `initMsg` failure needs no flag: it leaves `m.client` nil, so `refreshCmd` returns nil and no poll ever runs to wipe it.

## Actions without a backend

- When `backend.NewClient()` fails at startup (`initMsg`), the TUI keeps running with `m.client == nil`, and write actions reachable in that state must fail through `unavailableCmd` (`internal/ui/app.go`) - an ordinary `actionDoneMsg{err: "container CLI unavailable"}` through the one durable-error rendering path - instead of letting the action closure dereference the nil client inside its goroutine, which panics (`recordCmd` locks a field on a nil receiver). Guards sit at the three choke points every write funnels through: `runOnSelected`, `runAction` (covers `runGlobal`/`runPush`), and `confirmDelete`; prunes need no selection and typed prompts (pull, run form, volume create) need no items, so those are the live crash paths. Read paths were already nil-safe, and custom commands deliberately stay unguarded - they shell arbitrary commands, not the container CLI. `TestActionsWithoutClient_failNotPanic` drives all three prompt shapes; a PTY probe with `container` hidden from PATH verifies it end-to-end. Relatedly, main.go rejects unknown arguments with usage on stderr and exit 2 instead of silently opening the dashboard. Both behaviors were fixed once before (iteration 2 of the release-prep run) and silently lost in the rebase onto main - re-verify earlier-iteration fixes survive rebases rather than assuming they replayed.

## Detail-pane row budgets

- `uiutil.Pane.Add` charges each row against a fixed rendered-row budget and drops everything from the first row that doesn't fit onward (`internal/ui/uiutil/layout.go`); `uiutil.KV` gives a value no width bound, so a value long enough to wrap (a version string, a long path) can consume the whole remaining budget and silently drop every row after it, not just wrap ugly. Any value whose length isn't already bounded by the data (contrast a short "local"/"ext4" driver/format) must go through `KVFit`, not `KV`. Reproduce with the real pane width, not the terminal width: `internal/ui/app.go`'s `layoutDims` shrinks a 60-wide terminal's detail pane down to ~18 columns, and a value string short enough to fit a 60-wide test render can still overflow that.
- `container system status --format json` exits 1 while still printing a valid, parseable JSON body (`status: "unregistered"`) when the services have never been started - `backend.Client.runRaw` exists so that body isn't discarded the way `run`/`runJSON` discard stdout on any non-zero exit. `container system df --format json` has no such fallback: it prints a plain-text error on the same down state, so a services-down `DiskUsage` call is a real error for the caller to handle, even though the sibling `SystemStatus` call for the same state is not. See `internal/backend/system.go`.

## Container CLI sharp edges

- Vessel deliberately does NOT own registry login. A refused `image push` splits
  two ways in `internal/backend/images.go`: `credentialStderrPhrases` (401 and
  friends) tells the user to run `container registry login`;
  `permissionStderrPhrases` (403) names both possibilities — the account may
  lack write access, or the push may need a login — because a 403 does not on
  its own distinguish the two. Docker Hub and Google Artifact Registry both
  answer an *unauthenticated* push with 403 rather than 401, so a 403 is not
  proof the session is valid. Do not fold 403 back into the credential list or
  claim login is useless in its message — a 403 does not establish that the
  credentials were rejected, either.
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
  normal content, so the budget never drops it — but the pane's own height
  still clips it, and `PushPermissionNotice` already fills that height exactly
  at the smallest supported frame; the constant comments in
  `internal/backend/images.go` own that measurement, so check them before
  rewording a notice).
- On the installed 1.2.2 build (services running) `image save/load/tag/push` are
  core subcommands and `image pull` works live; honour the plugin gate only when
  a probe says so. `docs/APPLE_CONTAINER_MATRIX.md` records earlier probe results.
- `image tag <source> <target>` and `image save --output <path> <ref>` argument
  order is asserted in tests via `Client.CommandLog`; don't swap the order.
- The old "every invocation capped at 10s" bug is fixed: `runWithTimeout`
  (`internal/backend/client.go`) lets the known-long verbs pass a real budget,
  and the constants mirror internal/ui's outer bounds (see the budgets entry
  above). `image pull` joined the transfer set after shipping with the quick
  default that killed any real download mid-transfer; it now holds
  `globalTimeout` like its sibling transfer verbs.
- Digest-pinned image names are real CLI list output: after
  `container image pull <repo>@sha256:<digest>`, `container image list --format
  json` emits a row named exactly `<repo>@sha256:<digest>` (verified live on
  1.2.2; fixture at `internal/backend/testdata/images-digest.json`, and
  `image inspect` accepts the pinned ref too). `splitRef`/`FormatRef`
  (`internal/backend/images.go`) must round-trip that name byte-for-byte —
  `Image.Digest` is the reference digest, deliberately distinct from
  `ImageInspect.Digest` (the run-variant manifest digest). Tag/save/push still
  refuse digest-pinned rows (`ExactRef` + `imageActionRef`) because the CLI's
  acceptance of pinned sources for those verbs is unverified; the refusal keys
  off a non-empty `Image.Digest`, NOT off the tag, because `repo:tag@sha256:…`
  is a real list name that parses to a non-empty tag AND digest. Revisit only
  with a probe, and note push needs registry credentials to test fully.

## Release pipeline

- CI is least-privilege on purpose: `ci.yml` pins `permissions: contents: read`
  (artifact upload uses the runner's runtime token, not GITHUB_TOKEN, so read
  suffices) while `release.yml` keeps `contents: write` for goreleaser.
  `.github/dependabot.yml` opens weekly gomod + github-actions update PRs -
  expect them and treat them like any other PR (gate: make build/lint/test).
- Workflow concurrency asymmetry is deliberate: `ci.yml` sets
  `cancel-in-progress: true` (CI publishes nothing; superseded macos runs are
  pure 10x-rate waste) while `release.yml` queues overlapping runs with no
  cancel (a publish killed mid-flight can half-ship assets/formula). Every job
  in both workflows carries an explicit `timeout-minutes` so a hang fails fast
  instead of hitting the 360-minute runner default - including ci.yml's
  `graph` job (the required `update-go_modules-graph` context on PR heads,
  added on main after the Dependabot-only version was found to deadlock PR
  merges). Keep both when editing the workflows.
- `release.yml` pins goreleaser to an exact version (`v2.17.1`), not `latest`:
  goreleaser removes deprecated features at majors, so floating would silently
  cross the `brews:` removal cliff and fail tagging with zero repo changes.
  Dependabot cannot bump action *input* values, so version bumps are manual -
  run `goreleaser release --snapshot --clean`, verify the formula still lands
  at `dist/homebrew/Formula/vessel.rb`, then bump deliberately. ci.yml's
  golangci-lint `version: latest` stays floating on purpose: every PR exercises
  it, so breakage self-heals where pinning would silently rot.
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
- Tagged releases carry curated notes, not goreleaser's raw commit list: release.yml's
  "Generate release notes from CHANGELOG" step runs `scripts/release_notes.sh` and passes the
  file via `--release-notes=`. The script extracts the tag's own `## [x.y.z]` section,
  falls back to `[Unreleased]` when the version has no heading yet, and hard-fails when
  neither has content so a tag can never publish empty notes. Headings match by prefix
  (they carry ` - YYYY-MM-DD` suffixes; exact-line equality silently fell back to
  Unreleased in testing), and link-reference lines at the CHANGELOG tail terminate a
  section so they never leak into the oldest version's notes. goreleaser-action passes
  args to the binary WITHOUT shell expansion, so `$RUNNER_TEMP` inside `args:` would stay
  literal - the path travels through a step output (`steps.notes.outputs.path`) instead.
- `brews:` must set `directory: Formula` (confirmed against installed v2.17.1's own
  `goreleaser jsonschema` output - `folder` isn't even a valid property on that version).
  Without it, goreleaser writes the generated formula to the tap repo's root instead of
  `Formula/`. Homebrew resolves a tap's formulae from `Formula/` first, so a hand-written or
  older formula sitting in `Formula/` silently wins over a freshly published root-level one -
  a release can succeed while `brew install` keeps serving stale code. `goreleaser release
  --snapshot --clean` writes the formula to `dist/homebrew/Formula/vessel.rb` on this version;
  verify against that path (not `dist/homebrew/vessel.rb`) after touching this block.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
