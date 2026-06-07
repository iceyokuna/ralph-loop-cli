package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iceyokuna/ralph-loop-cli/internal/claude"
	"github.com/iceyokuna/ralph-loop-cli/internal/prompt"
	"github.com/iceyokuna/ralph-loop-cli/internal/state"
	"github.com/spf13/cobra"
)

// newPlanCmd builds the `ralph plan "<task>"` subcommand, which runs claude once
// to write requirements.md and implementation-plan.md into the target dir.
func newPlanCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   `plan "<task>"`,
		Short: "Run claude once to produce requirements.md and implementation-plan.md",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := claude.NewExecRunner(cmd.OutOrStdout())
			return runPlan(cmd.Context(), runner, opts, args[0])
		},
	}
}

// runPlan runs claude once with the planning prompt, records the run, and fails
// if claude cannot run or exits non-zero.
func runPlan(ctx context.Context, runner claude.Runner, opts *globalOptions, task string) error {
	o := claude.Options{
		Dir:    opts.Dir,
		Prompt: prompt.PlanPrompt(task),
		Model:  resolveModel(opts.Model, os.Getenv("RALPH_MODEL")),
		Perm:   opts.Perm,
	}

	started := time.Now()
	res, err := runner.Run(ctx, o)
	if err != nil {
		return fmt.Errorf("failed to run plan: %w", err)
	}
	recordPlanRun(opts.Dir, started, res)
	if res.ExitCode != 0 {
		return fmt.Errorf("claude exited with code %d", res.ExitCode)
	}
	return nil
}

// recordPlanRun records the plan invocation under .ralph/. It is best-effort:
// recording failures must not fail the plan, so errors are ignored.
func recordPlanRun(dir string, started time.Time, res claude.Result) {
	store := state.New(dir)
	if err := store.Ensure(); err != nil {
		return
	}
	_ = store.WriteTranscript(1, res.Output)
	_ = store.AppendRecord(state.Record{
		Index:          1,
		StartedAt:      started,
		FinishedAt:     time.Now(),
		ClaudeExitCode: res.ExitCode,
	})
}
