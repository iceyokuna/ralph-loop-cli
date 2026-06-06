// Package prompt builds the prompt strings ralph feeds to the claude CLI. They
// are plain strings, so they can be unit-tested directly.
package prompt

import "fmt"

// PlanPrompt returns the `ralph plan` prompt: it tells claude to turn task into
// requirements.md and implementation-plan.md (a "- [ ]" checklist) in the cwd.
func PlanPrompt(task string) string {
	return fmt.Sprintf(`You are helping plan a software task for an automated "Ralph loop".

The task is:

%s

Produce a clear, actionable plan by creating exactly two Markdown files in the
current working directory:

1. requirements.md
   - The functional requirements (what the software must do).
   - The non-functional requirements (constraints, quality attributes).
   - Concrete acceptance criteria.

2. implementation-plan.md
   - An ordered checklist of SMALL, independently testable tasks.
   - Use GitHub-style checkboxes ("- [ ]") for every task so progress can be
     tracked by checking them off.
   - Each task must be completable within a single loop iteration and should
     leave the build green and tests passing.

Write real, specific content tailored to the task above. Do not leave
placeholders or TODOs.`, task)
}

// DonePromise is the token claude emits once the plan is fully complete; ralph
// scans iteration output for it to detect completion.
const DonePromise = "<promise>RALPH_DONE</promise>"

// RunPrompt returns one `ralph run` iteration prompt: it embeds the requirements
// and plan and tells claude to implement the next unchecked task, emitting
// DonePromise only when everything is done.
func RunPrompt(reqs, plan string) string {
	return fmt.Sprintf(`You are an autonomous engineer running inside a "Ralph loop". This exact
prompt is fed to you on every iteration; your previous work persists on disk and
in git history. Do a SMALL, correct increment this iteration — never try to do
everything at once.

REQUIREMENTS (requirements.md):

%s

IMPLEMENTATION PLAN (implementation-plan.md):

%s

THIS ITERATION, DO EXACTLY THIS:
  1. Find the FIRST unchecked "- [ ]" task in the implementation plan above.
  2. Implement ONLY that one task. Make it real — no stubs, fakes, or TODOs.
  3. Keep the build green and the tests passing.
  4. Edit that task from "- [ ]" to "- [x]" in implementation-plan.md.

ONLY when every task in implementation-plan.md is checked off ("- [x]") AND the
build and tests genuinely pass, output the following token on its own line:

%s

Never output that token for any other reason — not to escape the loop, not when
stuck. A false completion is the one unforgivable error here.`, reqs, plan, DonePromise)
}
