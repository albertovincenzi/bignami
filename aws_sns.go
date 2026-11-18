package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var snsProduct = product{name: "SNS", desc: "topics, subscriptions, test message", icon: "◈", color: cAWS, items: snsItems}

func snsItems() []list.Item {
	a := acmd("◈")
	return []list.Item{
		a("Topics", "pick a topic, then an action", pickAWS("topics",
			[]string{"sns", "list-topics", "--query", "Topics[].TopicArn", "--output", "text"}, "◈", after(":"), snsTopic)),
		a("List topics (table)", "ARNs", do(`aws sns list-topics --query 'Topics[].TopicArn' --output table`)),
	}
}

func snsTopic(m *model, arn string) tea.Cmd {
	a := acmd("▸")
	m.push(after(":")(arn), []list.Item{
		a("Subscriptions", "protocol, endpoint, pending state", do(snsSubsScript(arn))),
		a("Filter and redrive policies", "per subscription", do(snsFiltersScript(arn))),
		a("Topic attributes", "policy, encryption, delivery", do(snsAttrsScript(arn))),
		a("Publish a test message", "reaches every subscriber", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Subject (optional)", optional: true}, {label: "Message"}}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This notifies every subscriber", after(":")(arn), func(m *model) tea.Cmd {
					return m.sh(snsPublishScript(arn, v[0], v[1]))
				})
			})
		}),
	})
	return nil
}

func snsSubsScript(arn string) string {
	return "aws sns list-subscriptions-by-topic --topic-arn " + shQuote(arn) +
		` --query 'Subscriptions[].[Protocol,Endpoint,SubscriptionArn]' --output table`
}

func snsFiltersScript(arn string) string {
	return `for S in $(aws sns list-subscriptions-by-topic --topic-arn ` + shQuote(arn) +
		` --query 'Subscriptions[].SubscriptionArn' --output text); do echo "== $S"; case "$S" in arn:*) aws sns get-subscription-attributes --subscription-arn "$S" --query 'Attributes.[FilterPolicy,RedrivePolicy]' --output text;; *) echo "(pending confirmation)";; esac; done`
}

func snsAttrsScript(arn string) string {
	return "aws sns get-topic-attributes --topic-arn " + shQuote(arn)
}

func snsPublishScript(arn, subject, message string) string {
	s := "aws sns publish --topic-arn " + shQuote(arn) + " --message " + shQuote(message)
	if subject != "" {
		s += " --subject " + shQuote(subject)
	}
	return s
}
