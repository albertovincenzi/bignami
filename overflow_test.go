package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The view must never be taller than the window, also with an error or an info line on screen:
// a taller view scrolls the header out of the terminal.
func TestViewFitsWithMessages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, msg := range []string{"", "err", "info", "busy"} {
		for _, size := range sizes() {
			m := newModel()
			m.w, m.h = size[0], size[1]
			m.sel = 0
			m.kubeCtx, m.awsProfile, m.awsRegion = "gke_acme-stage_europe-west1_k8s-03-stage", "alberto.vincenzi-mfa", "eu-west-1"
			m.kubeNS = "a-long-namespace-name"
			m.openDomain()
			m.push("SES", sesItems())
			switch msg {
			case "err":
				m.err = "exit status 254: aws: [ERROR]: An error occurred (AccessDeniedException) when calling the ListEmailIdentities operation: User: arn:aws:iam::123456789012:user/x is not authorized"
			case "info":
				m.info = "AWS region: eu-west-1"
			case "busy":
				m.busy = "loading identities..."
			}
			m.layout()
			if n := strings.Count(m.View(), "\n") + 1; n > m.h {
				t.Errorf("%s %v: view has %d lines, window has %d", msg, size, n, m.h)
			}
		}
	}
}

func sizes() [][2]int {
	var out [][2]int
	for w := 60; w <= 240; w += 10 {
		for h := 18; h <= 60; h++ {
			out = append(out, [2]int{w, h})
		}
	}
	return out
}

func TestSuppressedGetKeepsExitStatus(t *testing.T) {
	for stub, want := range map[string]struct {
		code int
		text string
	}{
		"echo 'An error occurred (NotFoundException)' >&2; exit 254":     {0, "not on the suppression list"},
		"echo 'An error occurred (AccessDeniedException)' >&2; exit 254": {254, "AccessDenied"},
		"echo '{\"Reason\":\"BOUNCE\"}'":                                 {0, "BOUNCE"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "aws"), []byte("#!/bin/sh\n"+stub+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", "-c", sesSuppressedGetScript("a@b.co"))
		cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin"}
		b, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if code != want.code || !strings.Contains(string(b), want.text) {
			t.Errorf("stub %q: exit %d, output %q", stub, code, b)
		}
	}
}
