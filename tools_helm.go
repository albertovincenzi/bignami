package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// helmCmd always carries the context and namespace, like kubectl in the TUI: the kubeconfig is never changed.
func helmCmd(ctx, ns string, args ...string) string {
	var pre []string
	if ctx != "" {
		pre = append(pre, "--kube-context", ctx)
	}
	if ns != "" {
		pre = append(pre, "-n", ns)
	}
	return shJoin("helm", append(pre, args...)...)
}

var helmReleasesProduct = product{name: "Releases", desc: "list, status, values, history, rollback", icon: "⎈", color: cHelm, items: helmReleaseItems}
var helmChartsProduct = product{name: "Charts", desc: "repos and chart search", icon: "▤", color: cHelm, items: helmChartItems}

func helmReleaseItems() []list.Item {
	a := cmdItem("⎈", cHelm)
	pick := func(title string, all bool) func(*model) tea.Cmd {
		return func(m *model) tea.Cmd {
			ctx, ns := m.kubeScope()
			args := []string{"list", "--short"}
			if all {
				args = append(args, "-A")
				ns = ""
			}
			return pickLines(title, helmCmd(ctx, ns, args...), "⎈", cHelm, func(f []string) (row, bool) {
				return row{name: f[0], val: f[0]}, f[0] != ""
			}, func(m *model, r row) tea.Cmd { return helmRelease(m, r.val) })(m)
		}
	}
	return []list.Item{
		a("Releases of the namespace", "pick one for status, values, history", pick("releases", false)),
		a("Releases in all namespaces", "pick one (uses the namespace of the TUI for the action)", pick("all releases", true)),
		a("Failed or pending", "every namespace", func(m *model) tea.Cmd {
			return m.plain(helmCmd(m.kubeCtx, "", "list", "-A", "--failed", "--pending"))
		}),
		a("Table of the namespace", "chart, version, status, updated", func(m *model) tea.Cmd { return m.plain(helmCmd(m.kubeCtx, m.kubeNS, "list")) }),
	}
}

func helmRelease(m *model, rel string) tea.Cmd {
	a := cmdItem("▸", cHelm)
	h := func(args ...string) func(*model) tea.Cmd {
		return func(m *model) tea.Cmd { return m.plain(helmCmd(m.kubeCtx, m.kubeNS, args...)) }
	}
	m.push(rel, []list.Item{
		a("Status", "resources and notes", h("status", rel)),
		a("History", "revisions", h("history", rel)),
		a("Values you set", "user-supplied values", h("get", "values", rel)),
		a("All values", "with the chart defaults", h("get", "values", rel, "--all")),
		a("Manifest", "the rendered YAML, the first 300 lines", func(m *model) tea.Cmd {
			return m.plain(helmCmd(m.kubeCtx, m.kubeNS, "get", "manifest", rel) + " | head -n 300")
		}),
		a("Roll back", "to a revision from History", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Revision number"}}, func(m *model, v []string) tea.Cmd {
				rev, err := digits("revision", v[0])
				if err != nil {
					m.err = err.Error()
					return nil
				}
				return m.guardProd(func(m *model) tea.Cmd {
					return m.confirmName("This rolls "+rel+" back to revision "+rev, rel, func(m *model) tea.Cmd {
						return m.plain(helmCmd(m.kubeCtx, m.kubeNS, "rollback", rel, rev))
					})
				})
			})
		}),
	})
	return nil
}

func helmChartItems() []list.Item {
	a := cmdItem("▤", cHelm)
	return []list.Item{
		a("Repos", "helm repo list", doPlain("helm repo list")),
		a("Search a chart", "in the repos you added", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Chart name"}}, func(m *model, v []string) tea.Cmd {
				return m.plain(shJoin("helm", "search", "repo", v[0]))
			})
		}),
	}
}
