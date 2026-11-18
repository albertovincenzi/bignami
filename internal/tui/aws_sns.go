package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertovincenzi/bignami/internal/scripts"
)

var snsProduct = product{name: "SNS", desc: "topics, subscriptions, test message", icon: "◈", color: cAWS, items: snsItems}

func snsItems() []list.Item {
	a := acmd("◈")
	return []list.Item{
		a("Topics", "pick a topic, then an action", pickAWS("topics",
			[]string{"sns", "list-topics", "--query", "Topics[].TopicArn", "--output", "text"}, "◈", scripts.After(":"), snsTopic)),
		a("List topics (table)", "ARNs", do(`aws sns list-topics --query 'Topics[].TopicArn' --output table`)),
	}
}

func snsTopic(m *model, arn string) tea.Cmd {
	a := acmd("▸")
	m.push(scripts.After(":")(arn), []list.Item{
		a("Subscriptions", "protocol, endpoint, pending state", do(scripts.SnsSubsScript(arn))),
		a("Filter and redrive policies", "per subscription", do(scripts.SnsFiltersScript(arn))),
		a("Topic attributes", "policy, encryption, delivery", do(scripts.SnsAttrsScript(arn))),
		a("Publish a test message", "reaches every subscriber", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Subject (optional)", optional: true}, {label: "Message"}}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This notifies every subscriber", scripts.After(":")(arn), func(m *model) tea.Cmd {
					return m.sh(scripts.SnsPublishScript(arn, v[0], v[1]))
				})
			})
		}),
	})
	return nil
}
