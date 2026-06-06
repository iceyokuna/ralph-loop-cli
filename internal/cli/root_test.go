package cli

import (
	"os"
	"testing"
)

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{name: "flag wins over env", flag: "opus", env: "sonnet", want: "opus"},
		{name: "flag wins with empty env", flag: "opus", env: "", want: "opus"},
		{name: "env used when flag empty", flag: "", env: "sonnet", want: "sonnet"},
		{name: "empty when both unset", flag: "", env: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RALPH_MODEL", tc.env)
			if got := resolveModel(tc.flag, os.Getenv("RALPH_MODEL")); got != tc.want {
				t.Errorf("resolveModel(%q, %q) = %q, want %q", tc.flag, tc.env, got, tc.want)
			}
		})
	}
}
