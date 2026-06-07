package prompt

import (
	"strings"
	"testing"
)

func TestPlanPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task string
	}{
		{name: "simple task", task: "build a TODO API"},
		{name: "task with punctuation", task: "write a CLI: parse flags & exit 0"},
		{name: "multiline task", task: "do A\nthen do B"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PlanPrompt(tc.task)

			// The task text must appear verbatim in the prompt.
			if !strings.Contains(got, tc.task) {
				t.Errorf("PlanPrompt(%q) must contain the task text; got:\n%s", tc.task, got)
			}

			// The prompt must instruct claude to write both planning files.
			for _, want := range []string{"requirements.md", "implementation-plan.md"} {
				if !strings.Contains(got, want) {
					t.Errorf("PlanPrompt must mention %q; got:\n%s", want, got)
				}
			}

			// The plan file must be described as a "- [ ]" checklist.
			if !strings.Contains(got, "- [ ]") {
				t.Errorf("PlanPrompt must instruct a \"- [ ]\" checklist; got:\n%s", got)
			}
		})
	}
}

func TestRunPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		reqs string
		plan string
	}{
		{name: "simple context", reqs: "must do X", plan: "- [ ] task one"},
		{name: "empty context", reqs: "", plan: ""},
		{name: "multiline context", reqs: "req A\nreq B", plan: "- [x] done\n- [ ] next"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RunPrompt(tc.reqs, tc.plan)

			// The requirements and plan context must appear verbatim.
			if tc.reqs != "" && !strings.Contains(got, tc.reqs) {
				t.Errorf("RunPrompt must embed the requirements text; got:\n%s", got)
			}
			if tc.plan != "" && !strings.Contains(got, tc.plan) {
				t.Errorf("RunPrompt must embed the plan text; got:\n%s", got)
			}

			// The completion token must appear literally.
			if !strings.Contains(got, DonePromise) {
				t.Errorf("RunPrompt must contain the %q token; got:\n%s", DonePromise, got)
			}
			if !strings.Contains(got, "<promise>RALPH_DONE</promise>") {
				t.Errorf("RunPrompt must contain the literal promise token; got:\n%s", got)
			}

			// The prompt must instruct implementing the next unchecked task.
			if !strings.Contains(got, "unchecked") {
				t.Errorf("RunPrompt must mention the \"unchecked\" task instruction; got:\n%s", got)
			}
			if !strings.Contains(got, "- [ ]") {
				t.Errorf("RunPrompt must reference the \"- [ ]\" task marker; got:\n%s", got)
			}
		})
	}
}
