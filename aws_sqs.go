package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	sqsMaxCounted   = 60 // queues whose message counts are fetched
	sqsParallelCall = 8
)

var sqsProduct = product{name: "SQS", desc: "depth, peek, DLQ redrive, purge", icon: "≣", color: cAWS, items: sqsItems}

func sqsItems() []list.Item {
	a := acmd("≣")
	return []list.Item{
		a("Queues", "with message counts; pick one for actions", func(m *model) tea.Cmd { return m.sqsQueues() }),
	}
}

// sqsQueues lists the queues with their message counts, fetched a few at a time.
func (m *model) sqsQueues() tea.Cmd {
	env, where := m.env(), m.regionLabel()
	return m.load("queues", func() ([]list.Item, error) {
		s, err := out(env, "aws", "sqs", "list-queues", "--query", "QueueUrls[]", "--output", "text")
		if err != nil {
			return nil, err
		}
		var urls []string
		for _, u := range strings.Fields(s) {
			if u != "None" {
				urls = append(urls, u)
			}
		}
		if len(urls) == 0 {
			return nil, fmt.Errorf("no queues found (%s)", where)
		}
		descs := make([]string, len(urls))
		visible := make([]string, len(urls))
		sem := make(chan struct{}, sqsParallelCall)
		var wg sync.WaitGroup
		for i, u := range urls[:min(len(urls), sqsMaxCounted)] {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				descs[i], visible[i] = queueCounts(env, u)
			}()
		}
		wg.Wait()
		var items []list.Item
		for i, u := range urls {
			name := after("/")(u)
			desc := descs[i]
			if desc == "" {
				desc = "counts not loaded"
			}
			items = append(items, item{title: name, desc: desc, icon: "≣", color: queueColor(name, visible[i]),
				act: func(m *model) tea.Cmd { return sqsQueue(m, u) }})
		}
		return items, nil
	})
}

func queueCounts(env []string, url string) (desc, visible string) {
	s, err := out(env, "aws", "sqs", "get-queue-attributes", "--queue-url", url,
		"--attribute-names", "ApproximateNumberOfMessages", "ApproximateNumberOfMessagesNotVisible", "ApproximateNumberOfMessagesDelayed",
		"--query", "Attributes.[ApproximateNumberOfMessages,ApproximateNumberOfMessagesNotVisible,ApproximateNumberOfMessagesDelayed]",
		"--output", "text")
	f := strings.Fields(s)
	if err != nil || len(f) != 3 {
		return "", ""
	}
	return fmt.Sprintf("visible %s · in flight %s · delayed %s", f[0], f[1], f[2]), f[0]
}

// queueColor flags a dead-letter queue that is not empty.
func queueColor(name, visible string) lipgloss.TerminalColor {
	lower := strings.ToLower(name)
	if (strings.Contains(lower, "dlq") || strings.Contains(lower, "dead")) && visible != "" && visible != "0" {
		return cErr
	}
	return cAWS
}

func sqsQueue(m *model, url string) tea.Cmd {
	a := acmd("▸")
	name := after("/")(url)
	m.push(name, []list.Item{
		a("Attributes", "all attributes, incl. redrive policy", do(sqsAttrsScript(url))),
		a("Peek at messages", "up to 10; raises the ReceiveCount", do(sqsPeekScript(url))),
		a("Send a test message", "a real consumer may process it", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Message body"}}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This adds a message to the queue", name, func(m *model) tea.Cmd { return m.sh(sqsSendScript(url, v[0])) })
			})
		}),
		a("Dead-letter source queues", "queues that use this one as DLQ", do(sqsDLQSourcesScript(url))),
		a("Redrive this DLQ", "moves messages back to the source", func(m *model) tea.Cmd {
			return m.confirmName("This moves every message out of the queue", name, func(m *model) tea.Cmd { return m.sh(sqsRedriveScript(url)) })
		}),
		a("Redrive status", "recent message move tasks", do(sqsRedriveStatusScript(url))),
		a("Purge", "deletes every message, no undo", func(m *model) tea.Cmd {
			return m.confirmName("This deletes every message", name, func(m *model) tea.Cmd { return m.sh(sqsPurgeScript(url)) })
		}),
	})
	return nil
}

func sqsAttrsScript(u string) string {
	return "aws sqs get-queue-attributes --queue-url " + shQuote(u) + " --attribute-names All"
}

func sqsPeekScript(u string) string {
	return "aws sqs receive-message --queue-url " + shQuote(u) +
		" --max-number-of-messages 10 --visibility-timeout 0 --message-system-attribute-names All --message-attribute-names All"
}

func sqsSendScript(u, body string) string {
	return "aws sqs send-message --queue-url " + shQuote(u) + " --message-body " + shQuote(body)
}

func sqsPurgeScript(u string) string {
	return "aws sqs purge-queue --queue-url " + shQuote(u) + ` && echo "purge started (it can take up to 60 seconds)"`
}

func sqsDLQSourcesScript(u string) string {
	return "aws sqs list-dead-letter-source-queues --queue-url " + shQuote(u)
}

func sqsArnVar(u string) string {
	return `A=$(aws sqs get-queue-attributes --queue-url ` + shQuote(u) + ` --attribute-names QueueArn --query Attributes.QueueArn --output text)`
}

func sqsRedriveScript(u string) string {
	return sqsArnVar(u) + `; aws sqs start-message-move-task --source-arn "$A"`
}

func sqsRedriveStatusScript(u string) string {
	return sqsArnVar(u) + `; aws sqs list-message-move-tasks --source-arn "$A"`
}
