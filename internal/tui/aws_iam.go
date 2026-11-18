package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertovincenzi/bignami/internal/awsprofile"
	"github.com/albertovincenzi/bignami/internal/scripts"
)

var iamProduct = product{name: "IAM", desc: "access, permission check, keys, roles", icon: "◍", color: cAWS, items: iamItems}

func iamItems() []list.Item {
	a := acmd("◍")
	return []list.Item{
		a("My access", "groups, attached and inline policies", do(scripts.IamMyAccessScript)),
		a("Can I do this?", "simulates an action, MFA-aware", func(m *model) tea.Cmd {
			mfa := strings.HasSuffix(m.awsProfile, awsprofile.MFASuffix)
			return m.ask([]promptSpec{{label: "Action, e.g. s3:GetObject"}, {label: "Resource ARN (default *)", optional: true}}, func(m *model, v []string) tea.Cmd {
				script, err := scripts.IamSimulateScript(v[0], v[1], mfa)
				if err != nil {
					m.err = err.Error()
					return nil
				}
				return m.sh(script)
			})
		}),
		a("My access keys", "status, age and last use", do(scripts.IamKeysScript)),
		a("My MFA devices", "serial numbers", do("aws iam list-mfa-devices --output table")),
		a("Roles", "trust policy and policies", pickAWS("roles",
			[]string{"iam", "list-roles", "--query", "Roles[].RoleName", "--output", "text"}, "◍", nil, iamRole)),
	}
}

func iamRole(m *model, role string) tea.Cmd {
	a := acmd("▸")
	m.push(role, []list.Item{
		a("Details", "trust policy and attached policies", do(scripts.IamRoleScript(role))),
	})
	return nil
}
