package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRunRunMissingPlanFiles verifies that `ralph run` surfaces a clear,
// path-naming error when the plan files are absent from the target dir, and
// that it fails before invoking the runner (FR-R1). A nil runner is safe here
// precisely because validation must short-circuit first.
func TestRunRunMissingPlanFiles(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		setup    func(t *testing.T)
		wantPath string
	}{
		{
			name:     "both files missing",
			setup:    func(t *testing.T) {},
			wantPath: "requirements.md",
		},
		{
			name: "implementation-plan.md missing",
			setup: func(t *testing.T) {
				if err := os.WriteFile(filepath.Join(dir, "requirements.md"), []byte("reqs"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantPath: "implementation-plan.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			opts := &globalOptions{Dir: dir}
			ro := &runOptions{Iterations: 1}

			err := runRun(context.Background(), nil, opts, ro)
			if err == nil {
				t.Fatal("runRun() = nil, want error for missing plan files")
			}
			if !strings.Contains(err.Error(), tc.wantPath) {
				t.Errorf("error %q does not name missing file %q", err, tc.wantPath)
			}
		})
	}
}

// TestRunCmdFlagDefaults asserts the load-bearing flag defaults: --iterations/-i
// is 1 and the inherited persistent --dir/-d is ".".
func TestRunCmdFlagDefaults(t *testing.T) {
	root := newRootCmd()

	var runCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "run" {
			runCmd = c
			break
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not registered on root")
	}

	iter := runCmd.Flags().Lookup("iterations")
	if iter == nil {
		t.Fatal(`run command missing "iterations" flag`)
	}
	if iter.Shorthand != "i" {
		t.Errorf("iterations shorthand = %q, want %q", iter.Shorthand, "i")
	}
	if iter.DefValue != "1" {
		t.Errorf("iterations default = %q, want %q", iter.DefValue, "1")
	}

	dir := runCmd.InheritedFlags().Lookup("dir")
	if dir == nil {
		t.Fatal(`run command missing inherited "dir" flag`)
	}
	if dir.Shorthand != "d" {
		t.Errorf("dir shorthand = %q, want %q", dir.Shorthand, "d")
	}
	if dir.DefValue != "." {
		t.Errorf("dir default = %q, want %q", dir.DefValue, ".")
	}
}
