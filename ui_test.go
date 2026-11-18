package main

import (
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Every screen must fit the terminal: no line wider than the window.
func TestViewsFitTerminal(t *testing.T) {
	for _, size := range [][2]int{{60, 24}, {80, 30}, {120, 40}, {200, 50}} {
		m := newModel()
		m.kubeCtx = "gke_acme-prod_europe-west1_k8s-03-prod"
		m.w, m.h = size[0], size[1]
		screens := map[string]func(){
			"picker": func() {},
			"aws":    func() { m.sel = 0; m.openDomain() },
			"k8s":    func() { m.stack = nil; m.sel = 1; m.openDomain() },
			"prompt": func() { m.prompt("PROD context - type 'prod'", false, nil) },
		}
		for _, name := range []string{"picker", "aws", "k8s", "prompt"} {
			screens[name]()
			v := m.View()
			for i, ln := range strings.Split(v, "\n") {
				if w := ansi.StringWidth(ln); w > m.w {
					t.Errorf("%s %v line %d is %d wide: %q", name, size, i, w, ansi.Strip(ln))
				}
			}
			if lines := strings.Count(v, "\n") + 1; lines > m.h {
				t.Errorf("%s %v has %d lines, window is %d", name, size, lines, m.h)
			}
		}
	}
}

// drain feeds the messages of a captured command into the model until it ends.
func drain(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for cmd != nil {
		done := make(chan tea.Msg, 1)
		go func() { done <- cmd() }()
		select {
		case msg := <-done:
			_, cmd = m.Update(msg)
		case <-deadline:
			t.Fatal("command did not finish")
		}
	}
}

func TestCaptureStreamsOutputAndStatus(t *testing.T) {
	m := newModel()
	m.w, m.h = 140, 40
	m.sel = 0
	m.openDomain()
	drain(t, m, m.capture(nil, `printf 'one\ntwo\n'; echo err >&2; exit 3`))
	if got := strings.Join(m.run.lines, ","); !strings.Contains(got, "one,two") || !strings.Contains(got, "err") {
		t.Errorf("lines = %q", got)
	}
	if !m.run.done || m.run.err == nil {
		t.Errorf("want a finished run with exit error, got done=%v err=%v", m.run.done, m.run.err)
	}
}

func TestStopKillsChildProcesses(t *testing.T) {
	m := newModel()
	m.w, m.h = 140, 40
	m.openDomain()
	cmd := m.capture(nil, `sleep 30 & sleep 30`)
	start := time.Now()
	m.stop()
	if !m.run.done || !errors.Is(m.run.err, errStopped) {
		t.Errorf("run not marked stopped: %+v", m.run)
	}
	_ = cmd
	if time.Since(start) > time.Second {
		t.Error("stop was slow")
	}
}

func TestPaneLayoutsFit(t *testing.T) {
	for _, size := range [][2]int{{70, 24}, {100, 30}, {110, 30}, {160, 45}} {
		m := newModel()
		m.w, m.h = size[0], size[1]
		m.openDomain()
		drain(t, m, m.capture(nil, `for i in 1 2 3; do echo "line $i with a very long tail xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"; done`))
		v := m.View()
		for i, ln := range strings.Split(v, "\n") {
			if w := ansi.StringWidth(ln); w > m.w {
				t.Errorf("%v line %d is %d wide: %q", size, i, w, ansi.Strip(ln))
			}
		}
		if n := strings.Count(v, "\n") + 1; n > m.h {
			t.Errorf("%v has %d lines, window is %d", size, n, m.h)
		}
		if !strings.Contains(ansi.Strip(v), "line 3") {
			t.Errorf("%v: output not visible", size)
		}
	}
}
