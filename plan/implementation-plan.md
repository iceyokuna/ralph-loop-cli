# ralph — Implementation Plan

Ordered, phased checklist. Every task is small and independently verifiable within a
single loop iteration. A verification step is stated per phase.

## Package layout
The suggested layout is kept as-is — it is the idiomatic Go split for this tool
(thin `cmd/` entrypoint; `internal/` packages each with one responsibility, which
keeps the loop core testable with a fake runner):

```
cmd/ralph/main.go             # entrypoint; calls cli.Execute()
internal/cli/root.go          # cobra root cmd, version, persistent flags
internal/cli/plan.go          # `ralph plan`
internal/cli/run.go           # `ralph run`
internal/claude/runner.go     # os/exec wrapper: build args, stream + capture stdout
internal/loop/engine.go       # loop core + completion check (token + gate)
internal/prompt/templates.go  # plan & run prompt templates
internal/state/state.go       # .ralph/ logs + per-iteration JSONL records
```

## Testing conventions & seams
Unit tests must run fast, offline, and WITHOUT invoking the real `claude` binary or
touching the real filesystem outside a temp dir. Build these seams up front so the
core logic is testable in isolation:

- **`claude.Runner` interface** — `internal/claude` exposes
  `type Runner interface { Run(ctx context.Context, o Options) (Result, error) }` with
  `Options{Dir, Prompt, Model string; Perm bool}` and `Result{Output string; ExitCode int}`.
  The production `ExecRunner` shells out via `os/exec`; tests inject a `fakeRunner` that
  returns scripted `Result`s and counts calls.
- **Pure `BuildArgs(Options) []string`** — claude arg construction is a standalone pure
  function (no exec), so it is table-tested directly.
- **Injectable gate** — the loop takes `Gate func(ctx context.Context, cmd string) error`
  (nil = no gate). Tests pass a closure returning `nil` / an error; one separate test
  exercises the real shell gate runner with `exit 0` / `exit 1`.
- **Filesystem isolation** — state tests use `t.TempDir()`; env tests use `t.Setenv`.
- **Style** — table-driven subtests via `t.Run(tc.name, …)`; assert behaviour and key
  invariants, not brittle full-string matches. Cover core logic (arg building, completion
  detection, loop control, state I/O); do not test cobra plumbing.

## Phase 0 — Scaffold + cobra + version
- [x] `go mod init github.com/iceyokuna/ralph-loop-cli`; add `spf13/cobra`.
- [x] Add `.gitignore` for `/ralph` (built binary) and `.ralph/`.
- [x] Create `cmd/ralph/main.go` calling `internal/cli.Execute()`.
- [x] Create `internal/cli/root.go` with the root cobra command and persistent flags
      `--dir/-d`, `--perm/-p`, `--model`.
- [x] Add `version`/`commit`/`date` vars (ldflags-injectable) and `--version` output.
- [x] Add `RALPH_MODEL` env fallback helper for model resolution.
- [x] **Test** `internal/cli/root_test.go`: table-driven `resolveModel(flag, env)`
      precedence — flag wins > `RALPH_MODEL` env > `""` — using `t.Setenv`.
- **Verify:** `go build -o ralph ./cmd/ralph && ./ralph --version` prints version;
  `go vet ./...` clean; `go test ./...` passes.

## Phase 1 — `plan` command
- [x] `internal/prompt/templates.go`: `PlanPrompt(task string)` instructing claude to
      write `requirements.md` + `implementation-plan.md` (with a `- [ ]` checklist)
      into the working dir.
- [x] `internal/claude/runner.go`: build claude args — ALWAYS `--print`; add
      `--dangerously-skip-permissions` only when perm=true; add `--model` when set;
      run via `os/exec` with cwd = `<dir>`, streaming stdout live and capturing it.
- [x] `internal/cli/plan.go`: wire `ralph plan "<task>" -d <dir>` to call the runner once.
- [x] **Test** `internal/claude/runner_test.go`: table-driven `BuildArgs` —
      `--print` ALWAYS present; perm=false omits / perm=true includes
      `--dangerously-skip-permissions`; model set adds `--model X`, empty omits it;
      the prompt is the final arg; `--dir` never leaks into the arg list.
- [x] **Test** `internal/prompt/templates_test.go`: `PlanPrompt(task)` contains the
      task text and instructs writing both `requirements.md` and `implementation-plan.md`.
- **Verify:** `go test ./...` green. Manual smoke:
  `ralph plan "tiny script" -d /tmp/p` creates the two files.

## Phase 2 — `run` loop core
- [x] `internal/cli/run.go`: add `--iterations/-i` (default 1); read + validate
      `requirements.md` and `implementation-plan.md` from `<dir>` (clear error if missing).
- [x] `internal/prompt/templates.go`: `RunPrompt(reqs, plan string)` = "implement the
      next unchecked task; emit `<promise>RALPH_DONE</promise>` only when truly done."
- [x] `internal/loop/engine.go`: loop up to N iterations, calling the runner each
      iteration; stream live; on claude error log + continue (still counts as an iteration).
- [x] Reaching N without completion → non-zero exit + clear message.
- [x] **Test** `internal/loop/engine_test.go` (with `fakeRunner`): never-complete run
      executes EXACTLY N iterations (assert `fakeRunner.calls == N`) and returns the
      incomplete outcome (exit≠0 / non-nil error).
- [x] **Test** claude-error path: `fakeRunner` returns an error on some iterations →
      engine logs and continues, the failed attempt still counts, and the loop reaches
      N (no early exit, no panic).
- [x] **Test** `internal/cli/run_test.go`: in a `t.TempDir()` missing the plan files,
      `run` returns a clear error; assert flag defaults (`-i`==1, `-d`==".").
- [x] **Test** `internal/prompt/templates_test.go`: `RunPrompt` contains the literal
      `<promise>RALPH_DONE</promise>` token and the "next unchecked task" instruction.
- **Verify:** `go test ./...` green. Manual smoke: `ralph run -d /tmp/p -i 2` performs
  2 passes.

## Phase 3 — Completion token + gate
- [x] Completion detector: scan captured stdout for `<promise>RALPH_DONE</promise>`.
- [x] Add `--gate` flag (run cmd only); on token-found, run gate via `os/exec`;
      gate exit 0 → complete; non-zero → keep looping.
- [x] Success path prints a summary + exit 0; integrate into `engine.go`.
- [x] **Test** `containsPromise` table cases: exact token present → true; absent → false;
      token embedded mid-line with surrounding text → true; partial/typo token → false;
      multiline output with the token on one line → true.
- [x] **Test** engine completion matrix (`fakeRunner` emits the token on iteration k):
      token + no-gate → done at k; token + gate-pass → done at k; token + gate-fail →
      keep looping (assert it does NOT stop at k and the gate was invoked); no-token →
      loops to N. Assert the completion iteration index and the gate-call count.
- [x] **Test** real gate runner: `RunGate(ctx, "exit 0")` returns nil;
      `RunGate(ctx, "exit 1")` returns a non-nil error.
- **Verify:** `go test ./...` green. Manual smoke with a stub `claude` script that
  emits the token (token+gate-fail keeps looping; token+gate-pass stops).

## Phase 4 — State / logging in `.ralph/`
- [x] `internal/state/state.go`: ensure `.ralph/` exists; append to `ralph.log`;
      write per-iteration transcript `iter-NNN.txt`; append a JSONL record to
      `iterations.jsonl` (`index, started_at, finished_at, claude_exit_code,
      promise_found, gate_ran, gate_exit_code`).
- [x] Wire state writes into the plan (single record) and run (per-iteration) paths.
- [x] **Test** `internal/state/state_test.go` (all under `t.TempDir()`): `Ensure()`
      creates `.ralph/`; `AppendRecord` writes one object per call and each line
      round-trips via `json.Unmarshal` with ALL fields populated (`index, started_at,
      finished_at, claude_exit_code, promise_found, gate_ran, gate_exit_code`); N appends
      → N lines; `WriteTranscript(i, data)` creates `iter-00i.txt` with the exact bytes;
      files use mode 0644 and the dir 0755.
- **Verify:** `go test ./...` green. Manual: inspect `.ralph/` after a real run.

## Phase 5 — Local build & install
- [x] Add a `Makefile` (or short build script) with `build`
      (`CGO_ENABLED=0 go build -ldflags "-X ...version=…" -o ralph ./cmd/ralph`),
      `install` (`go install ./cmd/ralph`), and `test` (`go test ./...`) targets.
- [x] Update the existing `README.md` with usage and build/install instructions
      (`go install ./cmd/ralph`; optional `GOOS`/`GOARCH` cross-compile note).
- **Verify:** `make test` is green; `go install ./cmd/ralph` puts `ralph` on `PATH`;
  `ralph --version` reflects the injected version. (No goreleaser/CI in v1.)

## Unit test matrix (summary)
| Component (file)                     | Test file                      | Key cases |
|--------------------------------------|--------------------------------|-----------|
| model resolution (`internal/cli`)    | `root_test.go`                 | flag > `RALPH_MODEL` > "" precedence |
| `BuildArgs` (`internal/claude`)      | `runner_test.go`               | `--print` always; `-p` toggles skip-perms; model set/unset; prompt last; no `--dir` leak |
| real gate runner (`internal/loop`)   | `engine_test.go`               | `exit 0`→nil, `exit 1`→error |
| `containsPromise` (`internal/loop`)  | `engine_test.go`               | present / absent / mid-line / partial / multiline |
| loop control (`internal/loop`)       | `engine_test.go` (`fakeRunner`)| runs N when never done; claude-error continues + counts; completion matrix (token×gate); gate-call count |
| prompts (`internal/prompt`)          | `templates_test.go`            | plan mentions both files + task; run embeds the promise token |
| state I/O (`internal/state`)         | `state_test.go` (`t.TempDir`)  | dir created; JSONL round-trips all fields; N appends→N lines; transcript bytes + file modes |
| run cmd guards (`internal/cli`)      | `run_test.go`                  | missing plan files → clear error; flag defaults `-i`=1, `-d`="." |

All unit tests run offline with `fakeRunner` / injected gate — none invoke the real
`claude` binary or the network.
