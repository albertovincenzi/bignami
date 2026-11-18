package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+$`)

var sesProduct = product{name: "SES", desc: "identities, suppressions, test email", icon: "✦", color: cAWS, items: sesItems}

func sesItems() []list.Item {
	a := acmd("✦")
	address := promptSpec{label: "Email address"}
	return []list.Item{
		a("Identities", "pick a domain or address", pickAWS("identities",
			[]string{"sesv2", "list-email-identities", "--query", "EmailIdentities[].IdentityName", "--output", "text"}, "✦", nil, sesIdentity)),
		a("Check the suppression list", "is this address blocked?", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{address}, func(m *model, v []string) tea.Cmd { return m.sh(sesSuppressedGetScript(v[0])) })
		}),
		a("Remove from the suppression list", "lets SES send to the address again", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{address}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This unblocks the address", v[0], func(m *model) tea.Cmd { return m.sh(sesSuppressedDeleteScript(v[0])) })
			})
		}),
		a("Recent suppressions", "last 30 blocked addresses and why", do(sesSuppressedList)),
		a("Account", "sandbox, quota, suppression settings", do(sesAccountScript)),
		a("Send a test email", "to an address you choose", sesSendFlow("")),
	}
}

func sesIdentity(m *model, id string) tea.Cmd {
	a := acmd("▸")
	from := ""
	if emailRe.MatchString(id) {
		from = id
	}
	m.push(id, []list.Item{
		a("Details", "verification, DKIM, mail-from", do(sesIdentityScript(id))),
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
			script, err := sesSendScript(f, v[0], v[1], v[2])
			if err != nil {
				m.err = err.Error()
				return nil
			}
			return m.confirmName("This sends a real email", v[0], func(m *model) tea.Cmd { return m.sh(script) })
		})
	}
}

func sesIdentityScript(id string) string {
	return "aws sesv2 get-email-identity --email-identity " + shQuote(id)
}

// sesSuppressedGetScript tells "not on the list" apart from a real error (no permission, no network), and keeps the exit status.
func sesSuppressedGetScript(addr string) string {
	return `O=$(aws sesv2 get-suppressed-destination --email-address ` + shQuote(addr) + ` 2>&1); S=$?
if [ $S -eq 0 ]; then echo "$O"; elif echo "$O" | grep -q NotFoundException; then echo "not on the suppression list"; else echo "$O"; exit $S; fi`
}

func sesSuppressedDeleteScript(addr string) string {
	return "aws sesv2 delete-suppressed-destination --email-address " + shQuote(addr) +
		` && echo "removed from the suppression list"`
}

const sesSuppressedList = `aws sesv2 list-suppressed-destinations --page-size 30 --query 'SuppressionListDestinationSummaries[].[EmailAddress,Reason,LastUpdateTime]' --output table`

const sesAccountScript = `aws sesv2 get-account --query '{InSandbox:!ProductionAccessEnabled,SendingEnabled:SendingEnabled,SendQuota:SendQuota,Suppression:SuppressionAttributes}'`

// sesSendScript builds the JSON arguments with encoding/json, so subject and body need no shell-shorthand escaping.
func sesSendScript(from, to, subject, body string) (string, error) {
	for _, a := range []string{from, to} {
		if !emailRe.MatchString(a) {
			return "", fmt.Errorf("not an email address: %q", a)
		}
	}
	content, err := json.Marshal(map[string]any{"Simple": map[string]any{
		"Subject": map[string]string{"Data": subject},
		"Body":    map[string]any{"Text": map[string]string{"Data": body}},
	}})
	if err != nil {
		return "", err
	}
	dest, err := json.Marshal(map[string][]string{"ToAddresses": {to}})
	if err != nil {
		return "", err
	}
	return strings.Join([]string{"aws sesv2 send-email",
		"--from-email-address", shQuote(from),
		"--destination", shQuote(string(dest)),
		"--content", shQuote(string(content))}, " "), nil
}
