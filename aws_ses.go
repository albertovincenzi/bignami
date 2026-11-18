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

var sesProduct = product{name: "SES", desc: "reputation, stats, identities, suppressions", icon: "✦", color: cAWS, items: sesItems}

func sesItems() []list.Item {
	a := acmd("✦")
	address := promptSpec{label: "Email address"}
	items := []list.Item{
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
	more := sesMoreItems() // reputation and statistics come first: they answer "is my sending healthy?"
	return append(append(more[:2:2], items...), more[2:]...)
}

func sesIdentity(m *model, id string) tea.Cmd {
	a := acmd("▸")
	from := ""
	if emailRe.MatchString(id) {
		from = id
	}
	m.push(id, []list.Item{
		a("Details", "verification, DKIM, mail-from", do(sesIdentityScript(id))),
		a("DKIM records", "the CNAMEs to publish in the DNS", do(sesDkimScript(id))),
		a("Sending policies", "who may send as this identity", do(sesPoliciesScript(id))),
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

// sesReputationScript shows the account state and the bounce and complaint rates of the last 7 days.
// AWS reviews an account at 5% bounces or 0.1% complaints, and pauses sending at 10% or 0.5%.
const sesReputationScript = `aws sesv2 get-account --query '{Enforcement:EnforcementStatus,SendingEnabled:SendingEnabled,ProductionAccess:ProductionAccessEnabled}'
S=$(date -u -v-7d +%FT%TZ 2>/dev/null || date -u -d '7 days ago' +%FT%TZ); E=$(date -u +%FT%TZ)
rate() { aws cloudwatch get-metric-statistics --namespace AWS/SES --metric-name "$1" --start-time "$S" --end-time "$E" --period 86400 --statistics Maximum --query 'sort_by(Datapoints,&Timestamp)[].[Timestamp,Maximum]' --output text | awk -v warn="$2" -v stop="$3" '{p=$2*100; f=(p>=stop)?"  PAUSE LEVEL":(p>=warn)?"  review level":""; printf "%s  %.3f%%%s\n", substr($1,1,10), p, f}'; }
echo; echo "bounce rate per day (review at 5%, pause at 10%):"; rate Reputation.BounceRate 5 10
echo; echo "complaint rate per day (review at 0.1%, pause at 0.5%):"; rate Reputation.ComplaintRate 0.1 0.5`

// sesStatsScript sums the send statistics of the last 2 weeks per day, with the 24 hour quota.
const sesStatsScript = `aws ses get-send-quota --query '{Max24HourSend:Max24HourSend,SentLast24Hours:SentLast24Hours,MaxSendRate:MaxSendRate}'
echo
aws ses get-send-statistics --query 'SendDataPoints[].[Timestamp,DeliveryAttempts,Bounces,Complaints,Rejects]' --output text | awk '{d=substr($1,1,10); a[d]+=$2; b[d]+=$3; c[d]+=$4; r[d]+=$5} END{for(d in a) printf "%s %d %d %d %d %.2f\n", d,a[d],b[d],c[d],r[d],(a[d]?b[d]*100/a[d]:0)}' | { printf "%-11s %8s %8s %10s %8s %8s\n" day sent bounces complaints rejects bounce%; sort | awk '{printf "%-11s %8d %8d %10d %8d %7.2f%%\n",$1,$2,$3,$4,$5,$6}'; }`

// sesSuppressedSummary counts the suppressed addresses per reason (the first 1000 of each).
const sesSuppressedSummary = `for R in BOUNCE COMPLAINT; do printf '%s: ' "$R"; aws sesv2 list-suppressed-destinations --reasons "$R" --page-size 1000 --query 'length(SuppressionListDestinationSummaries)' --output text; done`

const sesTemplatesScript = `aws sesv2 list-email-templates --query 'TemplatesMetadata[].[TemplateName,CreatedTimestamp]' --output table`

const sesConfigSetsScript = `aws sesv2 list-configuration-sets --query 'ConfigurationSets' --output text`

func sesConfigSetScript(name string) string {
	return "aws sesv2 get-configuration-set --configuration-set-name " + shQuote(name) +
		"; echo; echo '== event destinations (where bounces and complaints go)'; aws sesv2 get-configuration-set-event-destinations --configuration-set-name " + shQuote(name)
}

// sesDkimScript prints the CNAME records an Easy DKIM domain must publish.
func sesDkimScript(id string) string {
	return `I=` + shQuote(id) + `
aws sesv2 get-email-identity --email-identity "$I" --query 'DkimAttributes.[Status,SigningEnabled]' --output text
echo "records to publish in the DNS of $I:"
for T in $(aws sesv2 get-email-identity --email-identity "$I" --query 'DkimAttributes.Tokens' --output text); do [ "$T" = None ] || echo "${T}._domainkey.$I  CNAME  ${T}.dkim.amazonses.com"; done`
}

func sesPoliciesScript(id string) string {
	return "aws sesv2 get-email-identity-policies --email-identity " + shQuote(id)
}

func sesMoreItems() []list.Item {
	a := acmd("✦")
	return []list.Item{
		a("Reputation", "bounce and complaint rate, last 7 days", do(sesReputationScript)),
		a("Send statistics", "sent, bounces, complaints per day, quota", do(sesStatsScript)),
		a("Suppressions by reason", "bounces and complaints", do(sesSuppressedSummary)),
		a("Configuration sets", "pick one: event destinations", pickAWS("configuration sets",
			[]string{"sesv2", "list-configuration-sets", "--query", "ConfigurationSets", "--output", "text"}, "✦", nil, sesConfigSet)),
		a("Templates", "email templates", do(sesTemplatesScript)),
	}
}

func sesConfigSet(m *model, name string) tea.Cmd {
	a := acmd("▸")
	m.push(name, []list.Item{a("Details and event destinations", "where bounces and complaints are sent", do(sesConfigSetScript(name)))})
	return nil
}
