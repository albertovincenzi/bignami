package tui

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// runSh hands the terminal to a shell script. With pause, it waits for Enter
// at the end so the output stays visible. Exit codes are not reported: the
// command shows its own errors on the terminal.
func runSh(env []string, script string, pause bool) tea.Cmd {
	if pause {
		script += `; printf '\n[enter to return] '; read _`
	}
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = env
	return tea.ExecProcess(cmd, func(error) tea.Msg { return nil })
}
