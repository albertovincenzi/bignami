package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	cGit    = lipgloss.AdaptiveColor{Light: "#24292F", Dark: "#C9D1D9"}
	cHelm   = lipgloss.AdaptiveColor{Light: "#0F1689", Dark: "#7B83F5"}
	cGCP    = lipgloss.AdaptiveColor{Light: "#1A73E8", Dark: "#669DF6"}
	cDocker = lipgloss.AdaptiveColor{Light: "#0B7BC4", Dark: "#2496ED"}
)

var digitsRe = regexp.MustCompile(`^\d{1,12}$`)

// digits checks a number that goes into a command (PR number, run id, revision).
func digits(name, v string) (string, error) {
	if !digitsRe.MatchString(v) {
		return "", fmt.Errorf("%s must be a number", name)
	}
	return v, nil
}

// toolEnv is the environment of the commands that are not AWS: no prompts, no pagers.
func (m *model) toolEnv() []string {
	return append(os.Environ(), "GH_PAGER=cat", "GH_PROMPT_DISABLED=1", "CLOUDSDK_CORE_DISABLE_PROMPTS=1", "DOCKER_CLI_HINTS=false", "NO_COLOR=1")
}

func (m *model) plain(script string) tea.Cmd { return m.capture(m.toolEnv(), script) }

// doPlain runs a fixed script.
func doPlain(script string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd { return m.plain(script) }
}

// cmdItem builds a command item with the icon and color of its domain.
func cmdItem(icon string, col lipgloss.TerminalColor) func(title, desc string, act func(*model) tea.Cmd) item {
	return func(title, desc string, act func(*model) tea.Cmd) item {
		return item{title: title, desc: desc, icon: icon, color: col, act: act}
	}
}

// row is one line of a picker: name, a description and the value passed on.
type row struct {
	name, desc, val string
	color           lipgloss.TerminalColor
}

// pickLines runs a script that prints tab-separated lines, and shows them as a list.
func pickLines(title, script, icon string, col lipgloss.TerminalColor, parse func(f []string) (row, bool), onPick func(*model, row) tea.Cmd) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		env := m.toolEnv()
		return m.load(title, func() ([]list.Item, error) {
			s, err := out(env, "sh", "-c", script)
			if err != nil {
				return nil, err
			}
			var items []list.Item
			for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
				if ln == "" {
					continue
				}
				r, ok := parse(strings.Split(ln, "\t"))
				if !ok {
					continue
				}
				c := col
				if r.color != nil {
					c = r.color
				}
				items = append(items, item{title: r.name, desc: r.desc, icon: icon, color: c, act: func(m *model) tea.Cmd { return onPick(m, r) }})
			}
			if len(items) == 0 {
				return nil, fmt.Errorf("no %s found", title)
			}
			return items, nil
		})
	}
}

// field returns f[i], or "" when the line is shorter.
func field(f []string, i int) string {
	if i < len(f) {
		return f[i]
	}
	return ""
}

// kubeScope adds the context and namespace of the TUI to a tool that talks to the cluster.
func (m *model) kubeScope() (ctx, ns string) { return m.kubeCtx, m.kubeNS }
