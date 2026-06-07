// Package loop drives the Ralph loop: it repeatedly invokes claude through an
// injected Runner until the task completes or the iteration cap is hit. The
// Runner and gate are seams so the core can be tested without a real claude or
// shell.
package loop

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/iceyokuna/ralph-loop-cli/internal/claude"
	"github.com/iceyokuna/ralph-loop-cli/internal/prompt"
	"github.com/iceyokuna/ralph-loop-cli/internal/state"
)

// discardLogger drops all records; used when NewEngine is given a nil logger.
var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// Config describes a single loop run.
type Config struct {
	Iterations int            // max passes; below 1 runs none
	Options    claude.Options // claude invocation issued each pass
	Gate       string         // if set, must exit 0 (after the token) to complete
}

// Outcome reports how a loop run ended.
type Outcome struct {
	Iterations int  // passes performed
	Completed  bool // whether the loop completed before the cap
}

// Engine runs the Ralph loop against an injected Runner.
type Engine struct {
	logger *slog.Logger
	runner claude.Runner
	gate   func(ctx context.Context, cmd string) error
	store  *state.Store
}

// NewEngine returns an Engine that invokes runner each iteration and logs
// progress to logger (nil discards). Optional behaviour is added via SetGate and
// SetStore.
func NewEngine(logger *slog.Logger, runner claude.Runner) *Engine {
	if logger == nil {
		logger = discardLogger
	}
	logger = logger.With(slog.String("component", "loop"))
	return &Engine{logger: logger, runner: runner}
}

// SetGate sets the gate runner used to validate completion (nil uses RunGate).
func (e *Engine) SetGate(gate func(ctx context.Context, cmd string) error) {
	e.gate = gate
}

// SetStore sets the store that records each iteration's transcript and record.
func (e *Engine) SetStore(store *state.Store) {
	e.store = store
}

// Run loops up to cfg.Iterations passes. An iteration completes when claude's
// output contains the promise token and, if a gate is set, the gate exits 0; a
// failing gate keeps looping. A failed claude run is logged, still counts as a
// pass, and the loop continues (no retry).
func (e *Engine) Run(ctx context.Context, cfg Config) (Outcome, error) {
	var out Outcome
	for i := 1; i <= cfg.Iterations; i++ {
		out.Iterations = i
		e.logger.Info("invoking claude", slog.Int("iteration", i), slog.Int("max", cfg.Iterations))

		started := time.Now()
		res, runErr := e.runner.Run(ctx, cfg.Options)
		rec := state.Record{
			Index:          i,
			StartedAt:      started,
			FinishedAt:     time.Now(),
			ClaudeExitCode: res.ExitCode,
		}

		switch {
		case runErr != nil:
			rec.ClaudeExitCode = -1
			e.logger.Error("claude failed to run", slog.Int("iteration", i), slog.String("error", runErr.Error()))
		case res.ExitCode != 0:
			e.logger.Warn("claude exited non-zero", slog.Int("iteration", i), slog.Int("exit_code", res.ExitCode))
		default:
			e.logger.Info("claude exited", slog.Int("iteration", i), slog.Int("exit_code", res.ExitCode))
		}

		rec.PromiseFound = containsPromise(res.Output)
		completed := false
		if rec.PromiseFound {
			completed = e.checkGate(ctx, cfg.Gate, &rec)
		}

		e.record(rec, res.Output)

		if completed {
			out.Completed = true
			return out, nil
		}
	}
	return out, nil
}

// checkGate reports whether a token-emitting iteration completes: true with no
// gate, otherwise only if the gate exits 0. It fills rec's gate fields.
func (e *Engine) checkGate(ctx context.Context, gateCmd string, rec *state.Record) bool {
	if gateCmd == "" {
		e.logger.Info("promise found, no gate; completing", slog.Int("iteration", rec.Index))
		return true
	}

	rec.GateRan = true
	gate := e.gate
	if gate == nil {
		gate = RunGate
	}
	gateErr := gate(ctx, gateCmd)
	rec.GateExitCode = exitCode(gateErr)
	if gateErr == nil {
		e.logger.Info("promise found and gate passed; completing", slog.Int("iteration", rec.Index))
		return true
	}
	e.logger.Warn("promise found but gate failed; continuing",
		slog.Int("iteration", rec.Index), slog.Int("gate_exit_code", rec.GateExitCode))
	return false
}

// record writes the iteration's transcript and JSONL record when a store is set.
// Write failures are logged, never fatal.
func (e *Engine) record(rec state.Record, transcript string) {
	if e.store == nil {
		return
	}
	if err := e.store.WriteTranscript(rec.Index, transcript); err != nil {
		e.logger.Error("failed to write transcript", slog.Int("iteration", rec.Index), slog.String("error", err.Error()))
	}
	if err := e.store.AppendRecord(rec); err != nil {
		e.logger.Error("failed to write record", slog.Int("iteration", rec.Index), slog.String("error", err.Error()))
	}
}

// containsPromise reports whether s contains the completion token anywhere.
func containsPromise(s string) bool {
	return strings.Contains(s, prompt.DonePromise)
}

// RunGate runs cmd via "sh -c", returning its error (nil on exit 0). Output goes
// to stderr so failing-test details stay visible.
func RunGate(ctx context.Context, cmd string) error {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	return c.Run()
}

// exitCode maps a gate error to an exit code: 0 for nil, the real code for an
// *exec.ExitError, else 1.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}
