// Command ralph automates the "Ralph loop": it repeatedly invokes the claude
// CLI non-interactively until a task is complete.
package main

import "github.com/iceyokuna/ralph-loop-cli/internal/cli"

func main() {
	cli.Execute()
}
