package scripts

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/albertovincenzi/bignami/internal/shell"
)

var EmailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+$`)

func SesIdentityScript(id string) string {
	return "aws sesv2 get-email-identity --email-identity " + shell.Quote(id)
}

// SesSuppressedGetScript tells "not on the list" apart from a real error (no permission, no network), and keeps the exit status.
func SesSuppressedGetScript(addr string) string {
	return `O=$(aws sesv2 get-suppressed-destination --email-address ` + shell.Quote(addr) + ` 2>&1); S=$?
if [ $S -eq 0 ]; then echo "$O"; elif echo "$O" | grep -q NotFoundException; then echo "not on the suppression list"; else echo "$O"; exit $S; fi`
}

func SesSuppressedDeleteScript(addr string) string {
	return "aws sesv2 delete-suppressed-destination --email-address " + shell.Quote(addr) +
		` && echo "removed from the suppression list"`
}

const SesSuppressedList = `aws sesv2 list-suppressed-destinations --page-size 30 --query 'SuppressionListDestinationSummaries[].[EmailAddress,Reason,LastUpdateTime]' --output table`

const SesAccountScript = `aws sesv2 get-account --query '{InSandbox:!ProductionAccessEnabled,SendingEnabled:SendingEnabled,SendQuota:SendQuota,Suppression:SuppressionAttributes}'`

// SesSendScript builds the JSON arguments with encoding/json, so subject and body need no shell-shorthand escaping.
func SesSendScript(from, to, subject, body string) (string, error) {
	for _, a := range []string{from, to} {
		if !EmailRe.MatchString(a) {
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
		"--from-email-address", shell.Quote(from),
		"--destination", shell.Quote(string(dest)),
		"--content", shell.Quote(string(content))}, " "), nil
}

// sesReputationScript shows the account state and the bounce and complaint rates of the last 7 days.
// AWS reviews an account at 5% bounces or 0.1% complaints, and pauses sending at 10% or 0.5%.
const SesReputationScript = `aws sesv2 get-account --query '{Enforcement:EnforcementStatus,SendingEnabled:SendingEnabled,ProductionAccess:ProductionAccessEnabled}'
S=$(date -u -v-7d +%FT%TZ 2>/dev/null || date -u -d '7 days ago' +%FT%TZ); E=$(date -u +%FT%TZ)
rate() { aws cloudwatch get-metric-statistics --namespace AWS/SES --metric-name "$1" --start-time "$S" --end-time "$E" --period 86400 --statistics Maximum --query 'sort_by(Datapoints,&Timestamp)[].[Timestamp,Maximum]' --output text | awk -v warn="$2" -v stop="$3" '{p=$2*100; f=(p>=stop)?"  PAUSE LEVEL":(p>=warn)?"  review level":""; printf "%s  %.3f%%%s\n", substr($1,1,10), p, f}'; }
echo; echo "bounce rate per day (review at 5%, pause at 10%):"; rate Reputation.BounceRate 5 10
echo; echo "complaint rate per day (review at 0.1%, pause at 0.5%):"; rate Reputation.ComplaintRate 0.1 0.5`

// SesStatsScript sums the send statistics of the last 2 weeks per day, with the 24 hour quota.
const SesStatsScript = `aws ses get-send-quota --query '{Max24HourSend:Max24HourSend,SentLast24Hours:SentLast24Hours,MaxSendRate:MaxSendRate}'
echo
aws ses get-send-statistics --query 'SendDataPoints[].[Timestamp,DeliveryAttempts,Bounces,Complaints,Rejects]' --output text | awk '{d=substr($1,1,10); a[d]+=$2; b[d]+=$3; c[d]+=$4; r[d]+=$5} END{for(d in a) printf "%s %d %d %d %d %.2f\n", d,a[d],b[d],c[d],r[d],(a[d]?b[d]*100/a[d]:0)}' | { printf "%-11s %8s %8s %10s %8s %8s\n" day sent bounces complaints rejects bounce%; sort | awk '{printf "%-11s %8d %8d %10d %8d %7.2f%%\n",$1,$2,$3,$4,$5,$6}'; }`

// SesSuppressedSummary counts the suppressed addresses per reason (the first 1000 of each).
const SesSuppressedSummary = `for R in BOUNCE COMPLAINT; do printf '%s: ' "$R"; aws sesv2 list-suppressed-destinations --reasons "$R" --page-size 1000 --query 'length(SuppressionListDestinationSummaries)' --output text; done`

const SesTemplatesScript = `aws sesv2 list-email-templates --query 'TemplatesMetadata[].[TemplateName,CreatedTimestamp]' --output table`

const SesConfigSetsScript = `aws sesv2 list-configuration-sets --query 'ConfigurationSets' --output text`

func SesConfigSetScript(name string) string {
	return "aws sesv2 get-configuration-set --configuration-set-name " + shell.Quote(name) +
		"; echo; echo '== event destinations (where bounces and complaints go)'; aws sesv2 get-configuration-set-event-destinations --configuration-set-name " + shell.Quote(name)
}

// SesDkimScript prints the CNAME records an Easy DKIM domain must publish.
func SesDkimScript(id string) string {
	return `I=` + shell.Quote(id) + `
aws sesv2 get-email-identity --email-identity "$I" --query 'DkimAttributes.[Status,SigningEnabled]' --output text
echo "records to publish in the DNS of $I:"
for T in $(aws sesv2 get-email-identity --email-identity "$I" --query 'DkimAttributes.Tokens' --output text); do [ "$T" = None ] || echo "${T}._domainkey.$I  CNAME  ${T}.dkim.amazonses.com"; done`
}

func SesPoliciesScript(id string) string {
	return "aws sesv2 get-email-identity-policies --email-identity " + shell.Quote(id)
}
