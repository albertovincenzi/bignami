package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

func SnsSubsScript(arn string) string {
	return "aws sns list-subscriptions-by-topic --topic-arn " + shell.Quote(arn) +
		` --query 'Subscriptions[].[Protocol,Endpoint,SubscriptionArn]' --output table`
}

func SnsFiltersScript(arn string) string {
	return `for S in $(aws sns list-subscriptions-by-topic --topic-arn ` + shell.Quote(arn) +
		` --query 'Subscriptions[].SubscriptionArn' --output text); do echo "== $S"; case "$S" in arn:*) aws sns get-subscription-attributes --subscription-arn "$S" --query 'Attributes.[FilterPolicy,RedrivePolicy]' --output text;; *) echo "(pending confirmation)";; esac; done`
}

func SnsAttrsScript(arn string) string {
	return "aws sns get-topic-attributes --topic-arn " + shell.Quote(arn)
}

func SnsPublishScript(arn, subject, message string) string {
	s := "aws sns publish --topic-arn " + shell.Quote(arn) + " --message " + shell.Quote(message)
	if subject != "" {
		s += " --subject " + shell.Quote(subject)
	}
	return s
}
