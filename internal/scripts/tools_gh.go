package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

// gh works on the repository of the folder where bignami runs.
func GhCmd(repo string, args ...string) string {
	if repo != "" {
		args = append(args, "-R", repo)
	}
	return shell.Join("gh", args...)
}

func GhRunFailedScript(repo, id string) string {
	return GhCmd(repo, "run", "view", id, "--log-failed") + " 2>&1 | tail -n 300"
}
