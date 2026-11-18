package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var regionRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)

var awsRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-central-2", "eu-north-1", "eu-south-1", "eu-south-2",
	"ap-south-1", "ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2",
	"ca-central-1", "sa-east-1", "me-central-1", "af-south-1",
}

// acmd builds a command item with the icon of its product.
func acmd(icon string) func(title, desc string, act func(*model) tea.Cmd) item {
	return func(title, desc string, act func(*model) tea.Cmd) item {
		return item{title: title, desc: desc, icon: icon, color: cAWS, act: act}
	}
}

// env is the environment of every AWS command: selected profile and region.
func (m *model) env() []string { return awsEnv(m.awsProfile, m.awsRegion) }

// sh runs a script with the AWS environment and streams it into the output pane.
func (m *model) sh(script string) tea.Cmd { return m.capture(m.env(), script) }

// do runs a fixed script.
func do(script string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd { return m.sh(script) }
}

// pickAWS lists resources (one word each) and lets the user choose one.
func pickAWS(title string, args []string, icon string, shown func(string) string, onPick func(*model, string) tea.Cmd) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		env, where := m.env(), m.regionLabel()
		return m.load(title, func() ([]list.Item, error) {
			s, err := out(env, "aws", args...)
			if err != nil {
				return nil, err
			}
			var items []list.Item
			for _, v := range strings.Fields(s) {
				if v == "None" {
					continue
				}
				name := v
				if shown != nil {
					name = shown(v)
				}
				items = append(items, item{title: name, icon: icon, color: cAWS,
					act: func(m *model) tea.Cmd { return onPick(m, v) }})
			}
			if len(items) == 0 {
				return nil, fmt.Errorf("no %s found (%s)", title, where)
			}
			return items, nil
		})
	}
}

// after shows only what follows the last sep, for example the name at the end of an ARN.
func after(sep string) func(string) string {
	return func(s string) string {
		if i := strings.LastIndex(s, sep); i >= 0 {
			return s[i+len(sep):]
		}
		return s
	}
}

func (m *model) regionLabel() string {
	if m.awsRegion == "" {
		return "region: profile default"
	}
	return "region " + m.awsRegion
}

// back goes up one screen and shows a message.
func (m *model) back(info string) tea.Cmd {
	if len(m.stack) > 0 {
		m.stack = m.stack[:len(m.stack)-1]
	}
	m.info = info
	return nil
}

var sessionProduct = product{name: "Session", desc: "MFA login, profile, region, shell", icon: "●", color: cAWS, items: sessionItems}

func sessionItems() []list.Item {
	a := acmd("●")
	return []list.Item{
		a("MFA login", "12h session in profile <name>-mfa", mfaLogin),
		a("Who am I", "sts get-caller-identity", do("aws sts get-caller-identity")),
		a("Switch profile", "choose from ~/.aws/config", switchProfile),
		a("Switch region", "shortcut: r, on any AWS screen", switchRegion),
		a("Shell with this profile", "subshell with profile and region set", func(m *model) tea.Cmd {
			return runSh(m.env(), `exec "${SHELL:-sh}"`, false)
		}),
		a("Custom aws command", "aws <args>", func(m *model) tea.Cmd {
			return m.prompt("aws", false, func(m *model, v string) tea.Cmd {
				if v == "" {
					return nil
				}
				return m.sh("aws " + v)
			})
		}),
	}
}

func mfaLogin(m *model) tea.Cmd {
	base := strings.TrimSuffix(m.awsProfile, mfaSuffix)
	return m.prompt("MFA code for profile "+base, true, func(m *model, code string) tea.Cmd {
		m.busy = "logging in..."
		return func() tea.Msg {
			if err := awsLogin(base, code); err != nil {
				return errMsg{err}
			}
			return loginDoneMsg{base + mfaSuffix}
		}
	})
}

func switchProfile(m *model) tea.Cmd {
	return m.load("profiles", func() ([]list.Item, error) {
		ps, err := awsProfiles()
		if err != nil {
			return nil, err
		}
		var items []list.Item
		for _, p := range ps {
			items = append(items, item{title: p, icon: "●", color: cAWS, act: func(m *model) tea.Cmd {
				m.awsProfile = p
				return m.back("AWS profile: " + p)
			}})
		}
		return items, nil
	})
}

// switchRegion is also bound to the key r on every AWS screen.
func switchRegion(m *model) tea.Cmd {
	set := func(r string) func(*model) tea.Cmd {
		return func(m *model) tea.Cmd {
			m.awsRegion = r
			if r == "" {
				return m.back("AWS region: profile default")
			}
			return m.back("AWS region: " + r)
		}
	}
	var color lipgloss.TerminalColor = cAWS
	items := []list.Item{item{title: "(profile default)", desc: "use the region of the profile", icon: "◌", color: cMuted, act: set("")}}
	for _, r := range awsRegions {
		items = append(items, item{title: r, icon: "◌", color: color, act: set(r)})
	}
	m.push("regions", items)
	return nil
}
