package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertovincenzi/bignami/internal/scripts"
)

var sesProduct = product{name: "SES", desc: "reputation, stats, identities, suppressions", icon: "✦", color: cAWS, items: sesItems}

func sesItems() []list.Item {
	a := acmd("✦")
	address := promptSpec{label: "Email address"}
	items := []list.Item{
		a("Identities", "pick a domain or address", pickAWS("identities",
			[]string{"sesv2", "list-email-identities", "--query", "EmailIdentities[].IdentityName", "--output", "text"}, "✦", nil, sesIdentity)),
		a("Check the suppression list", "is this address blocked?", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{address}, func(m *model, v []string) tea.Cmd { return m.sh(scripts.SesSuppressedGetScript(v[0])) })
		}),
		a("Remove from the suppression list", "lets SES send to the address again", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{address}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This unblocks the address", v[0], func(m *model) tea.Cmd { return m.sh(scripts.SesSuppressedDeleteScript(v[0])) })
			})
		}),
		a("Recent suppressions", "last 30 blocked addresses and why", do(scripts.SesSuppressedList)),
		a("Account", "sandbox, quota, suppression settings", do(scripts.SesAccountScript)),
		a("Send a test email", "to an address you choose", sesSendFlow("")),
	}
	more := sesMoreItems() // reputation and statistics come first: they answer "is my sending healthy?"
	return append(append(more[:2:2], items...), more[2:]...)
}

func sesIdentity(m *model, id string) tea.Cmd {
	a := acmd("▸")
	from := ""
	if scripts.EmailRe.MatchString(id) {
		from = id
	}
	m.push(id, []list.Item{
		a("Details", "verification, DKIM, mail-from", do(scripts.SesIdentityScript(id))),
		a("DKIM records", "the CNAMEs to publish in the DNS", do(scripts.SesDkimScript(id))),
		a("Sending policies", "who may send as this identity", do(scripts.SesPoliciesScript(id))),
		a("Send a test email", "from this identity", sesSendFlow(from)),
	})
	return nil
}

// sesSendFlow asks for the recipient and the text, then confirms, because the email is real.
func sesSendFlow(from string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		specs := []promptSpec{{label: "To address"}, {label: "Subject"}, {label: "Message"}}
		if from == "" {
			specs = append([]promptSpec{{label: "From address (a verified identity)"}}, specs...)
		}
		return m.ask(specs, func(m *model, v []string) tea.Cmd {
			f := from
			if f == "" {
				f, v = v[0], v[1:]
			}
			script, err := scripts.SesSendScript(f, v[0], v[1], v[2])
			if err != nil {
				m.err = err.Error()
				return nil
			}
			return m.confirmName("This sends a real email", v[0], func(m *model) tea.Cmd { return m.sh(script) })
		})
	}
}

func sesMoreItems() []list.Item {
	a := acmd("✦")
	return []list.Item{
		a("Reputation", "bounce and complaint rate, last 7 days", do(scripts.SesReputationScript)),
		a("Send statistics", "sent, bounces, complaints per day, quota", do(scripts.SesStatsScript)),
		a("Suppressions by reason", "bounces and complaints", do(scripts.SesSuppressedSummary)),
		a("Configuration sets", "pick one: event destinations", pickAWS("configuration sets",
			[]string{"sesv2", "list-configuration-sets", "--query", "ConfigurationSets", "--output", "text"}, "✦", nil, sesConfigSet)),
		a("Templates", "email templates", do(scripts.SesTemplatesScript)),
	}
}

func sesConfigSet(m *model, name string) tea.Cmd {
	a := acmd("▸")
	m.push(name, []list.Item{a("Details and event destinations", "where bounces and complaints are sent", do(scripts.SesConfigSetScript(name)))})
	return nil
}
