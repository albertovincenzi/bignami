package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// out runs a command and returns stdout. On failure the error carries stderr.
func out(env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return string(b), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// shQuote quotes one argument for POSIX sh.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shJoin builds a shell command line with every argument quoted.
func shJoin(name string, args ...string) string {
	parts := []string{name}
	for _, a := range args {
		parts = append(parts, shQuote(a))
	}
	return strings.Join(parts, " ")
}

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
