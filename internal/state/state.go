// Package state manages ralph's per-project state under ".ralph/": a run log
// (ralph.log), one JSON record per iteration (iterations.jsonl), and a captured
// transcript per iteration (iter-NNN.txt).
package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const dirName = ".ralph"

// Store writes ralph's state into one project's ".ralph/" directory. Construct
// it with New.
type Store struct {
	dir string // path to the ".ralph/" directory
}

// New returns a Store for "<projectDir>/.ralph". Call Ensure before writing.
func New(projectDir string) *Store {
	return &Store{dir: filepath.Join(projectDir, dirName)}
}

// Record is one iterations.jsonl line: the outcome of a single iteration.
type Record struct {
	Index          int       `json:"index"`            // 1-based iteration number
	StartedAt      time.Time `json:"started_at"`       // claude invocation start
	FinishedAt     time.Time `json:"finished_at"`      // claude invocation end
	ClaudeExitCode int       `json:"claude_exit_code"` // -1 if claude failed to start
	PromiseFound   bool      `json:"promise_found"`    // completion token seen in stdout
	GateRan        bool      `json:"gate_ran"`         // gate executed this iteration
	GateExitCode   int       `json:"gate_exit_code"`   // gate exit code (0 if passed/not run)
}

// Ensure creates ".ralph/" (mode 0755) if needed.
func (s *Store) Ensure() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", s.dir, err)
	}
	return nil
}

// LogWriter opens ralph.log for appending; the caller must Close it.
func (s *Store) LogWriter() (io.WriteCloser, error) {
	path := filepath.Join(s.dir, "ralph.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}
	return f, nil
}

// AppendRecord appends r to iterations.jsonl as one JSON line.
func (s *Store) AppendRecord(r Record) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}
	return s.appendFile("iterations.jsonl", append(data, '\n'))
}

// WriteTranscript writes iteration i's captured stdout to iter-NNN.txt
// (zero-padded, mode 0644), replacing any prior transcript.
func (s *Store) WriteTranscript(i int, data string) error {
	path := filepath.Join(s.dir, fmt.Sprintf("iter-%03d.txt", i))
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// appendFile appends data to name inside the store dir, creating it if needed.
func (s *Store) appendFile(name string, data []byte) error {
	path := filepath.Join(s.dir, name)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
