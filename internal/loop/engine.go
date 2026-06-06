// Package loop drives the Ralph loop: it repeatedly invokes claude through an
// injected Runner until the task completes or the iteration cap is hit. The
// Runner and gate are seams so the core can be tested without a real claude or
// shell.
package loop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/iceyokuna/ralph-loop-cli/internal/claude"
	"github.com/iceyokuna/ralph-loop-cli/internal/prompt"
	"github.com/iceyokuna/ralph-loop-cli/internal/state"
)

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
	Runner claude.Runner                               // invokes claude each iteration
	Log    io.Writer                                   // progress lines; nil discards
	Gate   func(ctx context.Context, cmd string) error // gate runner; nil uses RunGate
	Store  *state.Store                                // if set, records each iteration
}

// Run loops up to cfg.Iterations passes. An iteration completes when claude's
// output contains the promise token and, if a gate is set, the gate exits 0; a
// failing gate keeps looping. A failed claude run is logged, still counts as a
// pass, and the loop continues (no retry).
func (e *Engine) Run(ctx context.Context, cfg Config) (Outcome, error) {
	var out Outcome
	for i := 1; i <= cfg.Iterations; i++ {
		out.Iterations = i
		e.logf("iteration %d/%d: invoking claude", i, cfg.Iterations)

		started := time.Now()
		res, runErr := e.Runner.Run(ctx, cfg.Options)
		rec := state.Record{
			Index:          i,
			StartedAt:      started,
			FinishedAt:     time.Now(),
			ClaudeExitCode: res.ExitCode,
		}

		switch {
		case runErr != nil:
			rec.ClaudeExitCode = -1
			e.logf("iteration %d: claude failed to run: %v (continuing)", i, runErr)
		case res.ExitCode != 0:
			e.logf("iteration %d: claude exited with code %d (continuing)", i, res.ExitCode)
		default:
			e.logf("iteration %d: claude exited 0", i)
		}

		rec.PromiseFound = containsPromise(res.Output)
		completed := false
		if rec.PromiseFound {
			completed = e.checkGate(ctx, cfg.Gate, i, &rec)
		}

		e.record(i, rec, res.Output)

		if completed {
			out.Completed = true
			return out, nil
		}
	}
	return out, nil
}

// checkGate reports whether a token-emitting iteration completes: true with no
// gate, otherwise only if the gate exits 0. It fills rec's gate fields.
func (e *Engine) checkGate(ctx context.Context, gateCmd string, i int, rec *state.Record) bool {
	if gateCmd == "" {
		e.logf("iteration %d: promise found, no gate — complete", i)
		return true
	}

	rec.GateRan = true
	gate := e.Gate
	if gate == nil {
		gate = RunGate
	}
	gateErr := gate(ctx, gateCmd)
	rec.GateExitCode = exitCode(gateErr)
	if gateErr == nil {
		e.logf("iteration %d: promise found and gate passed — complete", i)
		return true
	}
	e.logf("iteration %d: promise found but gate failed (exit %d) — continuing", i, rec.GateExitCode)
	return false
}

// record writes the iteration's transcript and JSONL record when a Store is set.
// Write failures are logged, never fatal.
func (e *Engine) record(i int, rec state.Record, transcript string) {
	if e.Store == nil {
		return
	}
	if err := e.Store.WriteTranscript(i, transcript); err != nil {
		e.logf("iteration %d: writing transcript: %v", i, err)
	}
	if err := e.Store.AppendRecord(rec); err != nil {
		e.logf("iteration %d: writing record: %v", i, err)
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

// logf writes a progress line when e.Log is set.
func (e *Engine) logf(format string, args ...any) {
	if e.Log == nil {
		return
	}
	fmt.Fprintf(e.Log, format+"\n", args...)
}
