# ralph — Requirements

## Overview
`ralph` is a single-binary Go CLI that automates the "Ralph loop" (Geoffrey
Huntley's bash-loop technique): it repeatedly invokes the `claude` CLI
(non-interactively, via `--print`) until a task is complete. `ralph` writes no
application code itself — it only orchestrates `claude`, streams its output, checks
for completion, and manages per-project state in `.ralph/`.

Module: `github.com/iceyokuna/ralph-loop-cli` (matches the GitHub remote). The built
binary / command is named `ralph` (from `cmd/ralph/`).

## Commands

### `ralph plan "<task>" -d <dir>`
- **FR-P1**: Accepts a single positional `<task>` string and a target dir
  (`-d`, default `.`).
- **FR-P2**: Invokes `claude` exactly ONCE with a planning prompt built from `<task>`.
- **FR-P3**: `--print` is always passed. If `-p/--perm` is set, also pass
  `--dangerously-skip-permissions`. If a model resolves, pass `--model`.
- **FR-P4**: The planning prompt instructs claude to create two files inside `<dir>`:
  - `requirements.md` — functional + non-functional requirements for the task.
  - `implementation-plan.md` — an ordered checklist of SMALL, testable tasks using
    `- [ ]`, each completable within a single loop iteration.
- **FR-P5**: Streams claude stdout live to the user and records the run under `.ralph/`.
- **FR-P6**: Exit 0 on success; non-zero with a clear message if claude fails to run.

### `ralph run -d <dir> -i <n> -p [--gate "<cmd>"] [--model <m>]`
- **FR-R1**: Reads `requirements.md` and `implementation-plan.md` from `<dir>`; errors
  with a clear message + non-zero exit if either is missing.
- **FR-R2**: Loops up to `<n>` iterations (`-i`, default 1). Each iteration invokes
  `claude` with an "implement the next unchecked task" prompt that includes the
  requirements + current plan context.
- **FR-R3**: `--print` always passed; `-p` adds `--dangerously-skip-permissions`;
  resolved model passed via `--model` when set.
- **FR-R4**: Completion is reached when claude stdout contains
  `<promise>RALPH_DONE</promise>` AND, if `--gate` is set, the gate command exits 0.
  The gate is evaluated only after the token is observed; gate non-zero → keep looping.
- **FR-R5**: On completion, print a success summary and exit 0.
- **FR-R6**: Reaching `<n>` iterations without completion → clear message + non-zero exit.
- **FR-R7**: On a single claude invocation error, log it and continue to the next
  iteration (no retry/backoff). A failed attempt still counts as one iteration.
- **FR-R8**: Each iteration streams claude stdout live while capturing it for the token
  scan and the transcript log.

## Flags
| flag           | short | default | applies | meaning |
|----------------|-------|---------|---------|---------|
| `--dir`        | `-d`  | `"."`   | both    | target project directory |
| `--iterations` | `-i`  | `1`     | run     | max loop iterations |
| `--perm`       | `-p`  | `false` | both    | pass `--dangerously-skip-permissions` to claude |
| `--gate`       |       | `""`    | run     | completion gate cmd, e.g. `go test ./...` |
| `--model`      |       | `""`    | both    | claude model override (falls back to `RALPH_MODEL`) |

## Claude invocation rules
- `--print` is ALWAYS present (non-interactive). This is independent of `-p`.
- `-p/--perm` ONLY toggles `--dangerously-skip-permissions`. The two are never conflated.
- Model precedence: `--model` flag > `RALPH_MODEL` env var > claude's own default.

## State & logging (`.ralph/` inside `<dir>`)
- `.ralph/ralph.log` — human-readable run log (appended).
- `.ralph/iterations.jsonl` — one JSON record per iteration:
  `{index, started_at, finished_at, claude_exit_code, promise_found, gate_ran, gate_exit_code}`.
- `.ralph/iter-NNN.txt` — full captured claude transcript for iteration NNN.
- `.ralph/` is created on demand and is meant to be git-ignorable per project.

## Non-functional requirements
- **NFR-1**: Single static binary; `CGO_ENABLED=0`.
- **NFR-2**: Builds with plain `go build`; cross-compiles to other OS/arch via
  `GOOS`/`GOARCH` when needed (no fixed release matrix or CI in v1).
- **NFR-3**: No network server, no daemon, no TUI, no plugin system, no parallel agents.
- **NFR-4**: Dependencies limited to stdlib + `spf13/cobra`.
- **NFR-5**: Installed locally via `go install ./cmd/ralph` (or a `go build` binary on
  `PATH`). No release automation, no goreleaser, no package-manager distribution in v1.
- **NFR-6**: `ralph --version` prints version/commit/date injected at build time.
- **NFR-7**: Idiomatic Go, standard project layout, table-driven tests where sensible.
- **NFR-8**: Core logic — claude arg building, completion-token detection, loop control,
  and `.ralph/` state I/O — is unit-tested with table-driven tests using injected fakes
  (a `Runner` interface + an injectable gate). Unit tests run offline and never invoke
  the real `claude` binary or the network; `go test ./...` is green.

## Acceptance criteria
- [ ] `ralph --version` prints an injected version string.
- [ ] `ralph plan "build a TODO API" -d ./demo` runs claude once and produces
      `demo/requirements.md` and `demo/implementation-plan.md`.
- [ ] `ralph run -d ./demo -i 5` loops, streaming claude output live, and stops early
      when `<promise>RALPH_DONE</promise>` is emitted.
- [ ] `ralph run -d ./demo -i 5 --gate "go test ./..."` only completes when the token
      is present AND the gate exits 0; a failing gate keeps it looping.
- [ ] Reaching the iteration cap without completion exits non-zero with a clear message.
- [ ] `-p` adds `--dangerously-skip-permissions`; `--print` is always passed regardless
      of `-p`.
- [ ] A claude error mid-loop is logged and the loop continues to the next iteration.
- [ ] `.ralph/` contains `ralph.log`, `iterations.jsonl`, and per-iteration transcripts.
- [ ] `go install ./cmd/ralph` puts a working `ralph` on `PATH`; `ralph --version` works.
- [ ] `go test ./...` passes; unit tests cover `BuildArgs`, completion detection, the
      loop completion matrix (token × gate), and the `.ralph/` JSONL round-trip — all
      offline with no real `claude`/network calls.
