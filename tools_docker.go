package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func dockerCmd(args ...string) string { return shJoin("docker", args...) }

var dockerContainersProduct = product{name: "Containers", desc: "logs, shell, stats, stop", icon: "▣", color: cDocker, items: dockerContainerItems}
var dockerImagesProduct = product{name: "Images and disk", desc: "images, disk usage, clean up", icon: "◫", color: cDocker, items: dockerImageItems}
var dockerComposeProduct = product{name: "Compose", desc: "services of this folder", icon: "☰", color: cDocker, items: dockerComposeItems}

func dockerContainerItems() []list.Item {
	a := cmdItem("▣", cDocker)
	pick := func(title string, all bool) func(*model) tea.Cmd {
		args := []string{"ps", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}"}
		if all {
			args = append([]string{"ps", "-a"}, args[1:]...)
		}
		return pickLines(title, dockerCmd(args...), "▣", cDocker, func(f []string) (row, bool) {
			col := lipgloss.TerminalColor(cOk)
			if len(f) > 1 && !hasPrefix(f[1], "Up") {
				col = cMuted
			}
			return row{name: f[0], desc: field(f, 1) + " · " + field(f, 2), val: f[0], color: col}, f[0] != ""
		}, func(m *model, r row) tea.Cmd { return dockerContainer(m, r.val) })
	}
	return []list.Item{
		a("Running", "pick one for logs, shell, stats", pick("containers", false)),
		a("All, with stopped", "pick one", pick("all containers", true)),
		a("Table", "docker ps", doPlain(dockerCmd("ps"))),
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func dockerShellScript(name string) string {
	return dockerCmd("exec", "-it", name, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
}

func dockerContainer(m *model, name string) tea.Cmd {
	a := cmdItem("▸", cDocker)
	m.push(name, []list.Item{
		a("Logs", "the last 200 lines", doPlain(dockerCmd("logs", "--tail", "200", name)+" 2>&1")),
		a("Shell", "bash or sh inside the container", func(m *model) tea.Cmd { return runSh(m.toolEnv(), dockerShellScript(name), false) }),
		a("Stats", "CPU and memory, one reading", doPlain(dockerCmd("stats", "--no-stream", name))),
		a("Inspect", "state, mounts, network, env (first 150 lines)", doPlain(dockerCmd("inspect", name)+" | head -n 150")),
		a("Restart", "docker restart", func(m *model) tea.Cmd {
			return m.confirmName("This restarts the container", name, func(m *model) tea.Cmd { return m.plain(dockerCmd("restart", name)) })
		}),
		a("Stop", "docker stop", func(m *model) tea.Cmd {
			return m.confirmName("This stops the container", name, func(m *model) tea.Cmd { return m.plain(dockerCmd("stop", name)) })
		}),
		a("Remove", "docker rm -f, the container is deleted", func(m *model) tea.Cmd {
			return m.confirmName("This deletes the container", name, func(m *model) tea.Cmd { return m.plain(dockerCmd("rm", "-f", name)) })
		}),
	})
	return nil
}

func dockerImageItems() []list.Item {
	a := cmdItem("◫", cDocker)
	return []list.Item{
		a("Images", "docker images", doPlain(dockerCmd("images"))),
		a("Disk usage", "images, containers, volumes, cache", doPlain(dockerCmd("system", "df"))),
		a("Dangling images", "untagged layers", doPlain(dockerCmd("images", "-f", "dangling=true"))),
		a("Remove dangling images", "docker image prune", func(m *model) tea.Cmd {
			return m.confirmName("This deletes the untagged images", "prune", func(m *model) tea.Cmd { return m.plain(dockerCmd("image", "prune", "-f")) })
		}),
		a("Clean all unused", "stopped containers, networks, unused images. Volumes stay", func(m *model) tea.Cmd {
			return m.confirmName("This deletes everything unused (not volumes)", "clean", func(m *model) tea.Cmd { return m.plain(dockerCmd("system", "prune", "-af")) })
		}),
	}
}

func dockerComposeItems() []list.Item {
	a := cmdItem("☰", cDocker)
	return []list.Item{
		a("Services", "docker compose ps", doPlain(dockerCmd("compose", "ps", "-a"))),
		a("Logs", "the last 100 lines of every service", doPlain(dockerCmd("compose", "logs", "--tail", "100", "--no-color")+" 2>&1")),
		a("Config check", "the merged file, or the error", doPlain(dockerCmd("compose", "config", "--quiet")+` && echo "compose file is valid"`)),
	}
}
