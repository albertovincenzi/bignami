package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

func SqsAttrsScript(u string) string {
	return "aws sqs get-queue-attributes --queue-url " + shell.Quote(u) + " --attribute-names All"
}

func SqsPeekScript(u string) string {
	return "aws sqs receive-message --queue-url " + shell.Quote(u) +
		" --max-number-of-messages 10 --visibility-timeout 0 --message-system-attribute-names All --message-attribute-names All"
}

func SqsSendScript(u, body string) string {
	return "aws sqs send-message --queue-url " + shell.Quote(u) + " --message-body " + shell.Quote(body)
}

func SqsPurgeScript(u string) string {
	return "aws sqs purge-queue --queue-url " + shell.Quote(u) + ` && echo "purge started (it can take up to 60 seconds)"`
}

func SqsDLQSourcesScript(u string) string {
	return "aws sqs list-dead-letter-source-queues --queue-url " + shell.Quote(u)
}

func SqsArnVar(u string) string {
	return `A=$(aws sqs get-queue-attributes --queue-url ` + shell.Quote(u) + ` --attribute-names QueueArn --query Attributes.QueueArn --output text)`
}

func SqsRedriveScript(u string) string {
	return SqsArnVar(u) + `; aws sqs start-message-move-task --source-arn "$A"`
}

func SqsRedriveStatusScript(u string) string {
	return SqsArnVar(u) + `; aws sqs list-message-move-tasks --source-arn "$A"`
}
