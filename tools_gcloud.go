package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// gcCmd adds the project chosen in the TUI as a flag, so the global gcloud config is not changed.
func gcCmd(project string, args ...string) string {
	if project != "" {
		args = append(args, "--project", project)
	}
	return shJoin("gcloud", args...)
}

var gcAccountProduct = product{name: "Account", desc: "who you are, project, login", icon: "☁", color: cGCP, items: gcAccountItems}
var gcGKEProduct = product{name: "GKE", desc: "clusters and kubeconfig credentials", icon: "⎈", color: cGCP, items: gcGKEItems}
var gcSecretsProduct = product{name: "Secrets", desc: "Secret Manager", icon: "▣", color: cGCP, items: gcSecretItems}
var gcLogsProduct = product{name: "Logs", desc: "Cloud Logging query", icon: "≣", color: cGCP, items: gcLogItems}

func gcAccountItems() []list.Item {
	a := cmdItem("☁", cGCP)
	return []list.Item{
		a("Who am I", "active account and project", func(m *model) tea.Cmd {
			return m.plain("gcloud auth list && gcloud config list --format='text(core.account,core.project)'")
		}),
		a("Switch project", "for this TUI only, gcloud config is not changed", pickLines("projects",
			shJoin("gcloud", "projects", "list", "--format=value(projectId,name)", "--sort-by=projectId", "--limit=200"), "☁", cGCP,
			func(f []string) (row, bool) { return row{name: f[0], desc: field(f, 1), val: f[0]}, f[0] != "" },
			func(m *model, r row) tea.Cmd {
				m.gcpProject = r.val
				return m.back("Google Cloud project: " + r.val)
			})),
		a("Log in", "opens the browser (gcloud auth login)", func(m *model) tea.Cmd { return runSh(m.toolEnv(), "gcloud auth login", true) }),
	}
}

func gcGKEItems() []list.Item {
	a := cmdItem("⎈", cGCP)
	return []list.Item{
		a("Clusters", "pick one to get its kubeconfig credentials", func(m *model) tea.Cmd {
			return pickLines("clusters", gcCmd(m.gcpProject, "container", "clusters", "list", "--format=value(name,location,status)"), "⎈", cGCP,
				func(f []string) (row, bool) {
					return row{name: f[0], desc: field(f, 1) + " · " + field(f, 2), val: field(f, 1)}, len(f) >= 2
				}, func(m *model, r row) tea.Cmd { return gcCluster(m, r.name, r.val) })(m)
		}),
		a("Clusters (table)", "plain list", func(m *model) tea.Cmd { return m.plain(gcCmd(m.gcpProject, "container", "clusters", "list")) }),
	}
}

func gcCluster(m *model, name, location string) tea.Cmd {
	a := cmdItem("▸", cGCP)
	m.push(name, []list.Item{
		a("Details", "version, node pools, endpoint", func(m *model) tea.Cmd {
			return m.plain(gcCmd(m.gcpProject, "container", "clusters", "describe", name, "--location", location))
		}),
		a("Get credentials", "ADDS a context to your kubeconfig", func(m *model) tea.Cmd {
			return m.confirmName("This adds a context to your kubeconfig", name, func(m *model) tea.Cmd {
				return m.plain(gcCmd(m.gcpProject, "container", "clusters", "get-credentials", name, "--location", location))
			})
		}),
	})
	return nil
}

func gcSecretItems() []list.Item {
	a := cmdItem("▣", cGCP)
	return []list.Item{
		a("Secrets", "pick one: versions, read the value", func(m *model) tea.Cmd {
			return pickLines("secrets", gcCmd(m.gcpProject, "secrets", "list", "--format=value(name)"), "▣", cGCP,
				func(f []string) (row, bool) { return row{name: after("/")(f[0]), val: after("/")(f[0])}, f[0] != "" },
				func(m *model, r row) tea.Cmd { return gcSecret(m, r.val) })(m)
		}),
	}
}

func gcSecret(m *model, name string) tea.Cmd {
	a := cmdItem("▸", cGCP)
	m.push(name, []list.Item{
		a("Versions", "state and creation date", func(m *model) tea.Cmd {
			return m.plain(gcCmd(m.gcpProject, "secrets", "versions", "list", name))
		}),
		a("Read the latest value", "shows the secret on screen", func(m *model) tea.Cmd {
			return m.confirmName("This prints the secret value", name, func(m *model) tea.Cmd {
				return m.plain(gcCmd(m.gcpProject, "secrets", "versions", "access", "latest", "--secret", name))
			})
		}),
	})
	return nil
}

func gcLogItems() []list.Item {
	a := cmdItem("≣", cGCP)
	return []list.Item{
		a("Read logs", "a Cloud Logging filter, the last hour, 50 entries", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Filter, e.g. severity>=ERROR"}}, func(m *model, v []string) tea.Cmd {
				return m.plain(gcLogsScript(m.gcpProject, v[0], "1h", "50"))
			})
		}),
	}
}

func gcLogsScript(project, filter, freshness, limit string) string {
	return gcCmd(project, "logging", "read", filter, "--freshness", freshness, "--limit", limit, "--format", "value(timestamp,severity,resource.labels.pod_name,textPayload,jsonPayload.message)")
}
