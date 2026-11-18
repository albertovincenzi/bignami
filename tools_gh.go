package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// gh works on the repository of the folder where bignami runs.
func ghCmd(repo string, args ...string) string {
	if repo != "" {
		args = append(args, "-R", repo)
	}
	return shJoin("gh", args...)
}

const ghPRFields = `.[]|[.number,.title,.author.login,.isDraft]|@tsv`
const ghRunFields = `.[]|[.databaseId,.displayTitle,.conclusion,.workflowName,.headBranch]|@tsv`

var ghPRProduct = product{name: "Pull requests", desc: "mine, to review, checks", icon: "⎇", color: cGit, items: ghPRItems}
var ghActionsProduct = product{name: "Actions", desc: "runs, failed logs, re-run", icon: "▶", color: cGit, items: ghActionsItems}
var ghRepoProduct = product{name: "Repo and issues", desc: "overview, releases, issues, auth", icon: "◉", color: cGit, items: ghRepoItems}

func ghPRItems() []list.Item {
	a := cmdItem("⎇", cGit)
	pick := func(title, search string) func(*model) tea.Cmd {
		return pickLines(title, ghCmd("", "pr", "list", "--limit", "30", "--search", search, "--json", "number,title,author,isDraft", "-q", ghPRFields), "⎇", cGit,
			func(f []string) (row, bool) {
				d := field(f, 2)
				if field(f, 3) == "true" {
					d += " · draft"
				}
				return row{name: "#" + f[0] + " " + field(f, 1), desc: d, val: f[0]}, len(f) >= 3
			}, func(m *model, r row) tea.Cmd { return ghPR(m, r.val, r.name) })
	}
	return []list.Item{
		a("My open PRs", "pick one for details and checks", pick("my PRs", "author:@me")),
		a("Waiting for my review", "review requested from me", pick("PRs to review", "review-requested:@me")),
		a("All open PRs", "the last 30", pick("open PRs", "")),
		a("Status of this branch", "PR of the current branch, checks, reviews", doPlain(ghCmd("", "pr", "status"))),
	}
}

func ghPR(m *model, n, title string) tea.Cmd {
	a := cmdItem("▸", cGit)
	m.push(title, []list.Item{
		a("Details", "description, reviews, status", doPlain(ghCmd("", "pr", "view", n))),
		a("Checks", "CI status of the PR", doPlain(ghCmd("", "pr", "checks", n))),
		a("Changed files", "names only", doPlain(ghCmd("", "pr", "diff", n, "--name-only"))),
		a("Comments", "the discussion", doPlain(ghCmd("", "pr", "view", n, "--comments"))),
	})
	return nil
}

func ghActionsItems() []list.Item {
	a := cmdItem("▶", cGit)
	pick := func(title string, extra ...string) func(*model) tea.Cmd {
		args := append([]string{"run", "list", "--limit", "20"}, extra...)
		args = append(args, "--json", "databaseId,displayTitle,conclusion,workflowName,headBranch", "-q", ghRunFields)
		return pickLines(title, ghCmd("", args...), "▶", cGit, func(f []string) (row, bool) {
			col := lipgloss.TerminalColor(cMuted)
			switch field(f, 2) {
			case "success":
				col = cOk
			case "failure":
				col = cErr
			}
			return row{name: field(f, 1), desc: field(f, 3) + " · " + field(f, 4) + " · " + field(f, 2), val: f[0], color: col}, len(f) >= 2
		}, func(m *model, r row) tea.Cmd { return ghRun(m, r.val, r.name) })
	}
	return []list.Item{
		a("Recent runs", "pick one: summary, failed logs, re-run", pick("runs")),
		a("Failed runs", "only the failures", pick("failed runs", "--status", "failure")),
		a("Recent runs (table)", "plain list", doPlain(ghCmd("", "run", "list", "--limit", "20"))),
	}
}

func ghRun(m *model, id, title string) tea.Cmd {
	a := cmdItem("▸", cGit)
	m.push(title, []list.Item{
		a("Summary", "jobs and steps", doPlain(ghCmd("", "run", "view", id))),
		a("Failed logs", "only the failed steps, the last 300 lines", doPlain(ghRunFailedScript("", id))),
		a("Re-run the failed jobs", "starts them again", func(m *model) tea.Cmd {
			return m.confirmName("This starts the failed jobs again", id, func(m *model) tea.Cmd { return m.plain(ghCmd("", "run", "rerun", id, "--failed")) })
		}),
	})
	return nil
}

func ghRunFailedScript(repo, id string) string {
	return ghCmd(repo, "run", "view", id, "--log-failed") + " 2>&1 | tail -n 300"
}

func ghRepoItems() []list.Item {
	a := cmdItem("◉", cGit)
	return []list.Item{
		a("Repo overview", "description and README", doPlain(ghCmd("", "repo", "view"))),
		a("Releases", "the last 10", doPlain(ghCmd("", "release", "list", "--limit", "10"))),
		a("My issues", "assigned to me", doPlain(ghCmd("", "issue", "list", "--assignee", "@me"))),
		a("Auth status", "which account gh uses", doPlain(ghCmd("", "auth", "status"))),
		a("Open in the browser", "gh browse", doPlain(ghCmd("", "browse"))),
	}
}
