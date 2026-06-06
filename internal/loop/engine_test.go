package loop

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iceyokuna/ralph-loop-cli/internal/claude"
	"github.com/iceyokuna/ralph-loop-cli/internal/prompt"
	"github.com/iceyokuna/ralph-loop-cli/internal/state"
)

// fakeRunner is a scripted claude.Runner for unit tests. It records how many
// times Run was called and returns Results produced by the script function,
// optionally failing on selected iterations. It never shells out.
type fakeRunner struct {
	// calls counts the number of Run invocations (1-based by the time script
	// observes its argument).
	calls int
	// script maps the 1-based call number to the Result and error returned.
	// A nil script yields an empty Result and nil error on every call.
	script func(call int) (claude.Result, error)
}

func (f *fakeRunner) Run(_ context.Context, _ claude.Options) (claude.Result, error) {
	f.calls++
	if f.script == nil {
		return claude.Result{}, nil
	}
	return f.script(f.calls)
}

func TestRunNeverCompletesRunsAllIterations(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{name: "single iteration", n: 1},
		{name: "several iterations", n: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &fakeRunner{}
			eng := &Engine{Runner: fr, Log: io.Discard}

			out, err := eng.Run(context.Background(), Config{Iterations: tc.n})
			if err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}
			if fr.calls != tc.n {
				t.Errorf("runner called %d times, want exactly %d", fr.calls, tc.n)
			}
			if out.Iterations != tc.n {
				t.Errorf("outcome.Iterations = %d, want %d", out.Iterations, tc.n)
			}
			if out.Completed {
				t.Error("outcome.Completed = true, want false for a never-completing run")
			}
		})
	}
}

func TestRunContinuesPastClaudeErrors(t *testing.T) {
	tests := []struct {
		name string
		n    int
		// fails reports whether the given 1-based call should return an error
		// (true) or a clean Result (false).
		fails func(call int) bool
	}{
		{
			name:  "error on first iteration",
			n:     3,
			fails: func(call int) bool { return call == 1 },
		},
		{
			name:  "error on a middle iteration",
			n:     4,
			fails: func(call int) bool { return call == 2 },
		},
		{
			name:  "every iteration fails",
			n:     3,
			fails: func(call int) bool { return true },
		},
		{
			name:  "alternating failures",
			n:     5,
			fails: func(call int) bool { return call%2 == 1 },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &fakeRunner{
				script: func(call int) (claude.Result, error) {
					if tc.fails(call) {
						return claude.Result{}, fmt.Errorf("claude unavailable on call %d", call)
					}
					return claude.Result{}, nil
				},
			}
			eng := &Engine{Runner: fr, Log: io.Discard}

			out, err := eng.Run(context.Background(), Config{Iterations: tc.n})
			if err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}
			if fr.calls != tc.n {
				t.Errorf("runner called %d times, want exactly %d (failed attempts must still count)", fr.calls, tc.n)
			}
			if out.Iterations != tc.n {
				t.Errorf("outcome.Iterations = %d, want %d", out.Iterations, tc.n)
			}
			if out.Completed {
				t.Error("outcome.Completed = true, want false when no completion occurred")
			}
		})
	}
}

func TestContainsPromise(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want bool
	}{
		{name: "exact token on its own", out: prompt.DonePromise, want: true},
		{name: "absent", out: "all done, nothing emitted", want: false},
		{name: "mid-line with surrounding text", out: "result: " + prompt.DonePromise + " ok", want: true},
		{name: "on one line of multiline output", out: "doing work\nfinishing up\n" + prompt.DonePromise + "\n", want: true},
		{name: "partial token", out: "<promise>RALPH_DON</promise>", want: false},
		{name: "typo token", out: "<promise>RALPH_DOOM</promise>", want: false},
		{name: "empty output", out: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsPromise(tc.out); got != tc.want {
				t.Errorf("containsPromise(%q) = %v, want %v", tc.out, got, tc.want)
			}
		})
	}
}

// TestRunCompletionMatrix exercises the token × gate completion logic. The fake
// runner emits the promise token on every iteration from completeAt onward; the
// gate is an injected closure whose pass/fail is fixed per case.
func TestRunCompletionMatrix(t *testing.T) {
	tests := []struct {
		name string
		n    int
		// completeAt is the first iteration whose output carries the token.
		completeAt int
		// gate: "" none, "pass" gate returns nil, "fail" gate returns error.
		gate string

		wantCompleted bool
		wantIters     int // passes performed (Outcome.Iterations)
		wantGateCalls int
	}{
		{
			name: "token, no gate, completes at k",
			n:    5, completeAt: 3, gate: "",
			wantCompleted: true, wantIters: 3, wantGateCalls: 0,
		},
		{
			name: "token + gate pass, completes at k",
			n:    5, completeAt: 2, gate: "pass",
			wantCompleted: true, wantIters: 2, wantGateCalls: 1,
		},
		{
			name: "token + gate fail keeps looping to N",
			n:    4, completeAt: 2, gate: "fail",
			// token emitted on iters 2,3,4 → gate tried each time, all fail.
			wantCompleted: false, wantIters: 4, wantGateCalls: 3,
		},
		{
			name: "no token loops to N",
			n:    3, completeAt: 99, gate: "pass",
			wantCompleted: false, wantIters: 3, wantGateCalls: 0,
		},
		{
			name: "token + gate pass on first iteration",
			n:    3, completeAt: 1, gate: "pass",
			wantCompleted: true, wantIters: 1, wantGateCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &fakeRunner{
				script: func(call int) (claude.Result, error) {
					out := "working...\n"
					if call >= tc.completeAt {
						out += prompt.DonePromise + "\n"
					}
					return claude.Result{Output: out}, nil
				},
			}

			gateCalls := 0
			var gate func(ctx context.Context, cmd string) error
			gate = func(_ context.Context, _ string) error {
				gateCalls++
				if tc.gate == "fail" {
					return fmt.Errorf("gate failed")
				}
				return nil
			}

			eng := &Engine{Runner: fr, Log: io.Discard, Gate: gate}
			out, err := eng.Run(context.Background(), Config{Iterations: tc.n, Gate: tc.gate})
			if err != nil {
				t.Fatalf("Run error: %v", err)
			}

			if out.Completed != tc.wantCompleted {
				t.Errorf("Completed = %v, want %v", out.Completed, tc.wantCompleted)
			}
			if out.Iterations != tc.wantIters {
				t.Errorf("Iterations = %d, want %d", out.Iterations, tc.wantIters)
			}
			if fr.calls != tc.wantIters {
				t.Errorf("runner called %d times, want %d", fr.calls, tc.wantIters)
			}
			if gateCalls != tc.wantGateCalls {
				t.Errorf("gate called %d times, want %d", gateCalls, tc.wantGateCalls)
			}
		})
	}
}

// TestRunRecordsState verifies the engine writes one transcript and one JSONL
// record per iteration into a real Store, with the per-iteration fields
// populated correctly: -1 exit on a claude start failure, promise detection, and
// the gate run/exit-code fields. It walks three iterations: (1) claude errors,
// (2) token + failing gate, (3) token + passing gate (completes).
func TestRunRecordsState(t *testing.T) {
	dir := t.TempDir()
	store := state.New(dir)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	fr := &fakeRunner{
		script: func(call int) (claude.Result, error) {
			switch call {
			case 1:
				return claude.Result{}, fmt.Errorf("claude unavailable")
			default:
				return claude.Result{Output: "work\n" + prompt.DonePromise + "\n"}, nil
			}
		},
	}

	gateCalls := 0
	gate := func(_ context.Context, _ string) error {
		gateCalls++
		if gateCalls == 1 { // first gate attempt (iteration 2) fails
			return fmt.Errorf("tests failed")
		}
		return nil
	}

	eng := &Engine{Runner: fr, Log: io.Discard, Gate: gate, Store: store}
	out, err := eng.Run(context.Background(), Config{Iterations: 3, Gate: "go test ./...", Options: claude.Options{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !out.Completed || out.Iterations != 3 {
		t.Fatalf("outcome = %+v, want Completed=true Iterations=3", out)
	}

	// Records: one JSON object per iteration, fields populated as expected.
	want := []state.Record{
		{Index: 1, ClaudeExitCode: -1, PromiseFound: false, GateRan: false, GateExitCode: 0},
		{Index: 2, ClaudeExitCode: 0, PromiseFound: true, GateRan: true, GateExitCode: 1},
		{Index: 3, ClaudeExitCode: 0, PromiseFound: true, GateRan: true, GateExitCode: 0},
	}
	data, err := os.ReadFile(filepath.Join(dir, ".ralph", "iterations.jsonl"))
	if err != nil {
		t.Fatalf("reading iterations.jsonl: %v", err)
	}
	var got []state.Record
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		if sc.Text() == "" {
			continue
		}
		var r state.Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("unmarshal %q: %v", sc.Text(), err)
		}
		got = append(got, r)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		// Compare the behavioural fields; timestamps are set at runtime.
		g := got[i]
		g.StartedAt, g.FinishedAt = want[i].StartedAt, want[i].FinishedAt
		if g != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	// One transcript per iteration.
	for i := 1; i <= 3; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".ralph", fmt.Sprintf("iter-%03d.txt", i))); err != nil {
			t.Errorf("missing transcript for iteration %d: %v", i, err)
		}
	}
}

// TestRunGate exercises the real shell gate runner (no claude, no network).
func TestRunGate(t *testing.T) {
	if err := RunGate(context.Background(), "exit 0"); err != nil {
		t.Errorf(`RunGate("exit 0") = %v, want nil`, err)
	}
	if err := RunGate(context.Background(), "exit 1"); err == nil {
		t.Error(`RunGate("exit 1") = nil, want non-nil error`)
	}
}
