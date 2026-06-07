# ralph-loop-cli

`ralph` is a tiny Go command-line tool that automates the **"Ralph loop"** —
Geoffrey Huntley's trick of running the `claude` CLI over and over until a coding
task is finished.

`ralph` writes no code itself. It just drives `claude`: it feeds it a prompt,
streams the output, checks whether the work is done, and keeps a log of every
attempt. Think of it as a patient operator that keeps asking `claude` to "do the
next step" until the whole task is complete.

## How it works

1. **Plan** — you describe a task once. `ralph` asks `claude` to break it into a
   `requirements.md` and an `implementation-plan.md` (a checklist of small steps).
2. **Run** — `ralph` calls `claude` in a loop. Each pass, `claude` does the next
   unchecked step and ticks it off.
3. **Stop** — the loop ends when `claude` prints the magic token
   `<promise>RALPH_DONE</promise>` (and, if you set a `--gate`, that command also
   passes — e.g. your tests are green). If it never finishes within the iteration
   limit, `ralph` exits with an error.

```
ralph plan "build a TODO API"   ->  requirements.md + implementation-plan.md
ralph run -i 10                 ->  loop: step, step, step ... done
```

## Requirements

- Go 1.24+ (to build).
- The [`claude` CLI](https://docs.claude.com/en/docs/claude-code) installed and
  on your `PATH` (`ralph` shells out to it).

## Install

```sh
go install github.com/iceyokuna/ralph-loop-cli/cmd/ralph@latest
```

Or from a clone of this repo:

```sh
make install     # go install, with version info baked in
make build       # build ./ralph in the current folder instead
```

Check it works:

```sh
ralph --version
```

## Quick start

```sh
# 1. Turn an idea into a plan (runs claude once)
ralph plan "build a small TODO REST API in Go" -d ./todo

# 2. Implement it, up to 10 passes, stopping when the tests pass
ralph run -d ./todo -i 10 --gate "go test ./..."
```

While `run` is looping you'll see `claude`'s output live, plus progress lines.
When it finishes you'll get `ralph: task complete after N iteration(s)`.

## Commands

### `ralph plan "<task>" -d <dir>`

Runs `claude` **once** to create two files in `<dir>`:

- `requirements.md` — what the software must do.
- `implementation-plan.md` — an ordered `- [ ]` checklist of small, testable steps.

### `ralph run -d <dir> -i <n>`

Loops `claude` over the plan in `<dir>`, up to `<n>` times. Each pass implements
the next unchecked step. The loop stops early when the task is complete (see
[How it works](#how-it-works)); reaching the limit first exits non-zero.

Add `--gate "<cmd>"` to require a command to pass before the run counts as done:

```sh
ralph run -d ./todo -i 10 --gate "go build ./... && go test ./..."
```

A failing gate keeps the loop going — handy for "don't stop until the build and
tests are green."

## Flags

| Flag           | Short | Default | Used by | Meaning                                           |
|----------------|-------|---------|---------|---------------------------------------------------|
| `--dir`        | `-d`  | `.`     | both    | target project directory                          |
| `--iterations` | `-i`  | `1`     | run     | maximum number of loop passes                     |
| `--gate`       |       | `""`    | run     | command that must exit 0 to finish (e.g. tests)   |
| `--perm`       | `-p`  | `false` | both    | pass `--dangerously-skip-permissions` to `claude` |
| `--model`      |       | `""`    | both    | `claude` model override                           |

Notes:

- `ralph` always runs `claude` non-interactively (`--print`). `-p` only controls
  whether `--dangerously-skip-permissions` is added.
- Model is chosen in this order: `--model` flag → `RALPH_MODEL` env var →
  `claude`'s own default.

## Where state is kept (`.ralph/`)

Each run records what happened in a `.ralph/` folder inside your `<dir>`:

- `ralph.log` — a structured log line for every iteration.
- `iterations.jsonl` — one JSON record per pass (timing, `claude`'s exit code,
  whether the token was found, gate result).
- `iter-001.txt`, `iter-002.txt`, … — the full `claude` output for each pass.

`.ralph/` is created automatically and is git-ignored.

## Development

```sh
make test    # run the unit tests (offline; never calls the real claude)
make vet     # go vet
make build   # build the binary
```
