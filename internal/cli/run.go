package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iceyokuna/ralph-loop-cli/internal/claude"
	"github.com/iceyokuna/ralph-loop-cli/internal/logger"
	"github.com/iceyokuna/ralph-loop-cli/internal/loop"
	"github.com/iceyokuna/ralph-loop-cli/internal/prompt"
	"github.com/iceyokuna/ralph-loop-cli/internal/state"
	"github.com/spf13/cobra"
)

// runOptions holds `ralph run`'s own (non-persistent) flags.
type runOptions struct {
	Iterations int    // --iterations/-i: max loop passes
	Gate       string // --gate: completion gate command; empty means none
}

// newRunCmd builds the `ralph run` subcommand: it loops claude over the plan in
// <dir> until completion or the iteration cap.
func newRunCmd(opts *globalOptions) *cobra.Command {
	ro := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Loop claude over implementation-plan.md until the task is complete",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := claude.NewExecRunner(cmd.OutOrStdout())
			return runRun(cmd.Context(), runner, opts, ro)
		},
	}

	cmd.Flags().IntVarP(&ro.Iterations, "iterations", "i", 1, "max loop iterations")
	cmd.Flags().StringVar(&ro.Gate, "gate", "", `completion gate command, e.g. "go test ./..."`)

	return cmd
}

// runRun validates the plan files exist, then drives the loop for up to
// ro.Iterations passes. Reaching the cap without completion is an error.
func runRun(ctx context.Context, runner claude.Runner, opts *globalOptions, ro *runOptions) error {
	reqs, plan, err := readPlanFiles(opts.Dir)
	if err != nil {
		return err
	}

	store := state.New(opts.Dir)
	if err := store.Ensure(); err != nil {
		return fmt.Errorf("failed to prepare state directory: %w", err)
	}
	logW, err := store.LogWriter()
	if err != nil {
		return fmt.Errorf("failed to open run log: %w", err)
	}
	defer logW.Close()

	log := logger.New(io.MultiWriter(os.Stderr, logW))
	engine := loop.NewEngine(log, runner)
	engine.SetStore(store)
	outcome, err := engine.Run(ctx, loop.Config{
		Iterations: ro.Iterations,
		Gate:       ro.Gate,
		Options: claude.Options{
			Dir:    opts.Dir,
			Prompt: prompt.RunPrompt(reqs, plan),
			Model:  resolveModel(opts.Model, os.Getenv("RALPH_MODEL")),
			Perm:   opts.Perm,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to run loop: %w", err)
	}
	if !outcome.Completed {
		return fmt.Errorf("reached iteration cap (%d) without completion", outcome.Iterations)
	}
	fmt.Fprintf(os.Stdout, "ralph: task complete after %d iteration(s)\n", outcome.Iterations)
	return nil
}

// readPlanFiles reads requirements.md and implementation-plan.md from dir; a
// missing or unreadable file yields an error naming the offending path.
func readPlanFiles(dir string) (reqs, plan string, err error) {
	reqsPath := filepath.Join(dir, "requirements.md")
	planPath := filepath.Join(dir, "implementation-plan.md")

	reqsBytes, err := os.ReadFile(reqsPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read %s: %w", reqsPath, err)
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read %s: %w", planPath, err)
	}
	return string(reqsBytes), string(planBytes), nil
}
