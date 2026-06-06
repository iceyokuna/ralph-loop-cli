package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Build metadata, injected via -ldflags (-X ...cli.version=...). The defaults
// apply to a plain go build/install.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// globalOptions holds the root command's persistent flags, shared by all
// subcommands.
type globalOptions struct {
	Dir   string // --dir/-d: target project directory
	Perm  bool   // --perm/-p: pass --dangerously-skip-permissions to claude
	Model string // --model: model override (falls back to RALPH_MODEL)
}

// newRootCmd builds the top-level ralph command and its persistent flags.
func newRootCmd() *cobra.Command {
	opts := &globalOptions{}

	cmd := &cobra.Command{
		Use:           "ralph",
		Short:         "Automate the Ralph loop around the claude CLI",
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetVersionTemplate("ralph {{.Version}}\n")

	pf := cmd.PersistentFlags()
	pf.StringVarP(&opts.Dir, "dir", "d", ".", "target project directory")
	pf.BoolVarP(&opts.Perm, "perm", "p", false, "pass --dangerously-skip-permissions to claude")
	pf.StringVar(&opts.Model, "model", "", "claude model override (falls back to RALPH_MODEL)")

	cmd.AddCommand(newPlanCmd(opts))
	cmd.AddCommand(newRunCmd(opts))

	return cmd
}

// resolveModel applies model precedence: --model flag, then RALPH_MODEL, else "".
func resolveModel(flag, env string) string {
	if flag != "" {
		return flag
	}
	return env
}

// Execute runs the root command, exiting non-zero on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ralph:", err)
		os.Exit(1)
	}
}
