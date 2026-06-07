// Package claude wraps invocation of the external claude CLI behind a Runner
// interface, so the rest of ralph can be tested without shelling out: production
// uses ExecRunner; tests inject a fake.
package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Options describes a single claude invocation.
type Options struct {
	Dir    string // working directory (cwd); never passed as an argument
	Prompt string // prompt text, passed positionally
	Model  string // passed via --model when non-empty
	Perm   bool   // passes --dangerously-skip-permissions when true
}

// Result captures the outcome of a claude invocation.
type Result struct {
	Output   string // full captured stdout
	ExitCode int    // process exit code (0 on success)
}

// Runner abstracts running claude so callers can be tested with a fake.
type Runner interface {
	Run(ctx context.Context, o Options) (Result, error)
}

// BuildArgs builds claude's argument list. Invariants (see requirements.md):
// --print is always present; --dangerously-skip-permissions only when Perm;
// --model X only when set; the prompt is last; Dir is never an argument.
func BuildArgs(o Options) []string {
	args := []string{"--print"}
	if o.Perm {
		args = append(args, "--dangerously-skip-permissions")
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	args = append(args, o.Prompt)
	return args
}

// ExecRunner is the production Runner: it shells out to the real claude binary.
type ExecRunner struct {
	stdout io.Writer // streamed claude stdout; defaults to os.Stdout
}

// NewExecRunner returns an ExecRunner that streams claude's stdout to stdout
// (nil defaults to os.Stdout).
func NewExecRunner(stdout io.Writer) *ExecRunner {
	return &ExecRunner{stdout: stdout}
}

// Run executes claude in o.Dir, streaming stdout to the configured writer while
// capturing it. A non-zero exit is reported via Result.ExitCode with a nil
// error; only start/wait failures return a non-nil error.
func (r *ExecRunner) Run(ctx context.Context, o Options) (Result, error) {
	out := r.stdout
	if out == nil {
		out = os.Stdout
	}

	cmd := exec.CommandContext(ctx, "claude", BuildArgs(o)...)
	cmd.Dir = o.Dir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(out, &buf)

	err := cmd.Run()
	res := Result{Output: buf.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return res, fmt.Errorf("failed to run claude: %w", err)
	}
	return res, nil
}
