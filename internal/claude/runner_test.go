package claude

import (
	"slices"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "minimal: print only",
			opts: Options{Prompt: "do the thing"},
			want: []string{"--print", "do the thing"},
		},
		{
			name: "perm adds skip-permissions",
			opts: Options{Prompt: "do the thing", Perm: true},
			want: []string{"--print", "--dangerously-skip-permissions", "do the thing"},
		},
		{
			name: "model is passed via --model",
			opts: Options{Prompt: "do the thing", Model: "opus"},
			want: []string{"--print", "--model", "opus", "do the thing"},
		},
		{
			name: "perm and model together",
			opts: Options{Prompt: "do the thing", Perm: true, Model: "sonnet"},
			want: []string{"--print", "--dangerously-skip-permissions", "--model", "sonnet", "do the thing"},
		},
		{
			name: "dir never leaks into args",
			opts: Options{Prompt: "do the thing", Dir: "/some/project/dir"},
			want: []string{"--print", "do the thing"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildArgs(tc.opts)

			if !slices.Equal(got, tc.want) {
				t.Fatalf("BuildArgs(%+v) = %q, want %q", tc.opts, got, tc.want)
			}

			// --print is ALWAYS present, independent of Perm.
			if !slices.Contains(got, "--print") {
				t.Errorf("--print must always be present, got %q", got)
			}

			// --dangerously-skip-permissions appears iff Perm is set.
			hasSkip := slices.Contains(got, "--dangerously-skip-permissions")
			if hasSkip != tc.opts.Perm {
				t.Errorf("skip-permissions present = %v, want %v (Perm=%v)", hasSkip, tc.opts.Perm, tc.opts.Perm)
			}

			// --model appears iff a model is set.
			hasModel := slices.Contains(got, "--model")
			if hasModel != (tc.opts.Model != "") {
				t.Errorf("--model present = %v, want %v", hasModel, tc.opts.Model != "")
			}

			// The prompt is always the final argument.
			if len(got) == 0 || got[len(got)-1] != tc.opts.Prompt {
				t.Errorf("prompt must be the final arg; got %q, want last = %q", got, tc.opts.Prompt)
			}

			// Dir must never appear as an argument.
			if tc.opts.Dir != "" && slices.Contains(got, tc.opts.Dir) {
				t.Errorf("Dir %q leaked into args %q", tc.opts.Dir, got)
			}
		})
	}
}
