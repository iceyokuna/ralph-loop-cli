package state

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEnsureCreatesDir verifies Ensure creates ".ralph/" with mode 0755 and is
// idempotent.
func TestEnsureCreatesDir(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	if err := s.Ensure(); err != nil {
		t.Fatalf("Ensure() error: %v", err)
	}
	// Calling again must not error.
	if err := s.Ensure(); err != nil {
		t.Fatalf("second Ensure() error: %v", err)
	}

	info, err := os.Stat(filepath.Join(root, dirName))
	if err != nil {
		t.Fatalf("stat .ralph: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".ralph is not a directory")
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf(".ralph mode = %o, want 0755", perm)
	}
}

// TestAppendRecordRoundTrips verifies that each appended Record is one JSON line
// that round-trips with all fields populated, and that N appends yield N lines.
func TestAppendRecordRoundTrips(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.Ensure(); err != nil {
		t.Fatalf("Ensure() error: %v", err)
	}

	started := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	want := []Record{
		{Index: 1, StartedAt: started, FinishedAt: started.Add(time.Minute), ClaudeExitCode: 0, PromiseFound: false, GateRan: false, GateExitCode: 0},
		{Index: 2, StartedAt: started.Add(2 * time.Minute), FinishedAt: started.Add(3 * time.Minute), ClaudeExitCode: 2, PromiseFound: true, GateRan: true, GateExitCode: 1},
		{Index: 3, StartedAt: started.Add(4 * time.Minute), FinishedAt: started.Add(5 * time.Minute), ClaudeExitCode: 0, PromiseFound: true, GateRan: true, GateExitCode: 0},
	}

	for _, r := range want {
		if err := s.AppendRecord(r); err != nil {
			t.Fatalf("AppendRecord(%d) error: %v", r.Index, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(root, dirName, "iterations.jsonl"))
	if err != nil {
		t.Fatalf("reading iterations.jsonl: %v", err)
	}

	var got []Record
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		got = append(got, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning records: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d (one JSON object per append)", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d round-trip mismatch:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}

	// Spot-check that all snake_case keys are present on the wire.
	for _, key := range []string{
		"index", "started_at", "finished_at", "claude_exit_code",
		"promise_found", "gate_ran", "gate_exit_code",
	} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("iterations.jsonl missing JSON key %q", key)
		}
	}
}

// TestWriteTranscript verifies the per-iteration transcript file name, exact
// bytes, and mode.
func TestWriteTranscript(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.Ensure(); err != nil {
		t.Fatalf("Ensure() error: %v", err)
	}

	const body = "line one\nline two\n"
	if err := s.WriteTranscript(7, body); err != nil {
		t.Fatalf("WriteTranscript error: %v", err)
	}

	path := filepath.Join(root, dirName, "iter-007.txt")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat transcript: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("transcript mode = %o, want 0644", perm)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading transcript: %v", err)
	}
	if string(got) != body {
		t.Errorf("transcript bytes = %q, want %q", got, body)
	}
}

// TestLogWriterAppends verifies LogWriter opens ralph.log for appending and that
// writes accumulate across separate writers.
func TestLogWriterAppends(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if err := s.Ensure(); err != nil {
		t.Fatalf("Ensure() error: %v", err)
	}

	for _, chunk := range []string{"alpha\n", "beta\n"} {
		w, err := s.LogWriter()
		if err != nil {
			t.Fatalf("LogWriter error: %v", err)
		}
		if _, err := io.WriteString(w, chunk); err != nil {
			t.Fatalf("write error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("close error: %v", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(root, dirName, "ralph.log"))
	if err != nil {
		t.Fatalf("reading ralph.log: %v", err)
	}
	if string(data) != "alpha\nbeta\n" {
		t.Errorf("ralph.log = %q, want %q", data, "alpha\nbeta\n")
	}
}
