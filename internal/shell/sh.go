package shell

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Output runs a command and returns stdout. On failure the error carries stderr.
func Output(env []string, name string, args ...string) (string, error) {
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

func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Quote quotes one argument for POSIX sh.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Join builds a shell command line with every argument quoted.
func Join(name string, args ...string) string {
	parts := []string{name}
	for _, a := range args {
		parts = append(parts, Quote(a))
	}
	return strings.Join(parts, " ")
}
