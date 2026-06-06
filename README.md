# ralph-loop-cli

`ralph` is a single-binary Go CLI that automates the **Ralph loop** (Geoffrey
Huntley's bash-loop technique): it repeatedly invokes the `claude` CLI
non-interactively (via `--print`) until a task is complete. `ralph` writes no
application code itself — it only orchestrates `claude`, streams its output,
checks for completion, and manages per-project state in `.ralph/`.

## Install

```sh
go install github.com/iceyokuna/ralph-loop-cli/cmd/ralph@latest
# or, from a clone:
make install        # go install with version ldflags
make build          # build ./ralph in the working tree
```

Cross-compile by setting `GOOS`/`GOARCH`, e.g. `GOOS=linux GOARCH=amd64 make build`.

```sh
ralph --version     # prints version (commit, built date) injected at build time
```

## Usage

### Plan

Run `claude` once to turn a task into `requirements.md` and
`implementation-plan.md` (a `- [ ]` checklist of small, testable steps):

```sh
ralph plan "build a TODO API" -d ./demo
```

### Run

Loop `claude` over the plan until done or the iteration cap is reached:

```sh
ralph run -d ./demo -i 5
```

Each iteration tells `claude` to implement the next unchecked task. The loop
completes when `claude` emits `<promise>RALPH_DONE</promise>` in its output. With
a **gate**, completion also requires the gate command to exit 0 — a failing gate
keeps the loop going:

```sh
ralph run -d ./demo -i 5 --gate "go test ./..."
```

Reaching the iteration cap without completion exits non-zero with a clear
message.

## Flags

| flag           | short | default | applies | meaning                                              |
|----------------|-------|---------|---------|------------------------------------------------------|
| `--dir`        | `-d`  | `"."`   | both    | target project directory                             |
| `--iterations` | `-i`  | `1`     | run     | max loop iterations                                  |
| `--perm`       | `-p`  | `false` | both    | pass `--dangerously-skip-permissions` to `claude`    |
| `--gate`       |       | `""`    | run     | completion gate command, e.g. `go test ./...`        |
| `--model`      |       | `""`    | both    | model override (falls back to `RALPH_MODEL`)         |

`--print` is **always** passed to `claude`; `-p` only toggles
`--dangerously-skip-permissions`. Model precedence: `--model` > `RALPH_MODEL` >
`claude`'s own default.

## State (`.ralph/` inside `<dir>`)

- `ralph.log` — human-readable run log (appended).
- `iterations.jsonl` — one JSON record per iteration (`index`, `started_at`,
  `finished_at`, `claude_exit_code`, `promise_found`, `gate_ran`,
  `gate_exit_code`).
- `iter-NNN.txt` — full captured `claude` transcript for iteration NNN.

`.ralph/` is created on demand and is git-ignored.

## Development

```sh
make test    # offline unit tests; never invokes the real claude or network
make vet
```
