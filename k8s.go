package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func isProd(ctx string) bool { return strings.Contains(ctx, "prod") }

// kargs builds kubectl arguments that always carry the selected context and
// namespace. The TUI never changes the global kubeconfig.
func (m *model) kargs(args ...string) []string {
	var a []string
	if m.kubeCtx != "" {
		a = append(a, "--context", m.kubeCtx)
	}
	if m.kubeNS != "" {
		a = append(a, "-n", m.kubeNS)
	}
	return append(a, args...)
}

// guardProd asks for confirmation before a risky action in a prod context.
func (m *model) guardProd(next func(*model) tea.Cmd) tea.Cmd {
	if !isProd(m.kubeCtx) {
		return next(m)
	}
	return m.prompt("PROD context "+m.kubeCtx+" - type 'prod' to continue", false,
		func(m *model, v string) tea.Cmd {
			if v != "prod" {
				m.err = "cancelled"
				return nil
			}
			return next(m)
		})
}

func (m *model) k8sContexts() tea.Cmd {
	return m.load("contexts", func() ([]list.Item, error) {
		s, err := out(nil, "kubectl", "config", "get-contexts", "-o", "name")
		if err != nil {
			return nil, err
		}
		var items []list.Item
		for _, c := range strings.Fields(s) {
			desc := ""
			if isProd(c) {
				desc = "PROD"
			}
			items = append(items, item{title: c, desc: desc, icon: "◈", color: ctxColor(c), act: func(m *model) tea.Cmd {
				m.kubeCtx, m.kubeNS = c, ""
				return m.home("context: " + c)
			}})
		}
		return items, nil
	})
}

func (m *model) k8sNamespaces() tea.Cmd {
	argv := m.kargs("get", "ns", "-o", "jsonpath={.items[*].metadata.name}")
	return m.load("namespaces", func() ([]list.Item, error) {
		s, err := out(nil, "kubectl", argv...)
		if err != nil {
			return nil, err
		}
		items := []list.Item{item{title: "(context default)", act: func(m *model) tea.Cmd {
			m.kubeNS = ""
			return m.home("namespace: context default")
		}}}
		for _, ns := range strings.Fields(s) {
			items = append(items, item{title: ns, icon: iconK8s, color: cK8s, act: func(m *model) tea.Cmd {
				m.kubeNS = ns
				return m.home("namespace: " + ns)
			}})
		}
		return items, nil
	})
}

func (m *model) k8sPods() tea.Cmd {
	argv := m.kargs("get", "pods", "--no-headers")
	return m.load("pods", func() ([]list.Item, error) {
		s, err := out(nil, "kubectl", argv...)
		if err != nil {
			return nil, err
		}
		var items []list.Item
		for _, ln := range strings.Split(s, "\n") {
			f := strings.Fields(ln)
			if len(f) < 3 {
				continue
			}
			pod := f[0]
			items = append(items, item{title: pod, desc: strings.Join(f[1:], " "), icon: "●", color: podColor(f[2]),
				act: func(m *model) tea.Cmd { return m.podMenu(pod) }})
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("no pods found")
		}
		return items, nil
	})
}

func (m *model) podMenu(pod string) tea.Cmd {
	m.push("pod "+pod, []list.Item{
		item{title: "Logs (follow)", icon: "≡", color: cK8s, desc: "last 200 lines, x to stop", act: func(m *model) tea.Cmd {
			return m.capture(nil, shJoin("kubectl", m.kargs("logs", "-f", "--tail=200", pod)...))
		}},
		item{title: "Shell", icon: "❯", color: cK8s, desc: "bash if present, else sh", act: func(m *model) tea.Cmd {
			return m.guardProd(func(m *model) tea.Cmd {
				return runSh(nil, shJoin("kubectl", m.kargs("exec", "-it", pod, "--",
					"sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")...), false)
			})
		}},
		item{title: "Describe", icon: "◉", color: cK8s, desc: "full pod description", act: func(m *model) tea.Cmd {
			return m.capture(nil, shJoin("kubectl", m.kargs("describe", "pod", pod)...))
		}},
	})
	return nil
}

func (m *model) kubectlPrompt() tea.Cmd {
	return m.prompt("kubectl (context "+m.kubeCtx+")", false, func(m *model, v string) tea.Cmd {
		if v == "" {
			return nil
		}
		return m.guardProd(func(m *model) tea.Cmd {
			return m.capture(nil, shJoin("kubectl", m.kargs()...)+" "+v)
		})
	})
}

func ctxColor(c string) lipgloss.TerminalColor {
	if isProd(c) {
		return cErr
	}
	return cK8s
}

func podColor(status string) lipgloss.TerminalColor {
	switch status {
	case "Running":
		return cOk
	case "Completed", "Succeeded":
		return cMuted
	case "Pending", "ContainerCreating", "PodInitializing":
		return cWarn
	}
	return cErr
}
