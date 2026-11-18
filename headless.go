package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// A headless command is one action without the TUI, for scripts and for agents (bignami run, bignami mcp).
// It reuses the script builders of the TUI, so both paths run the same shell.
type argSpec struct {
	name, desc string
	optional   bool
}

type cmdSpec struct {
	name, desc string
	args       []argSpec
	write      bool                             // changes data: needs --allow-write and the confirm value
	confirm    func(a map[string]string) string // the value the caller must repeat in "confirm"
	build      func(a map[string]string) (string, error)
}

func req(name, desc string) argSpec { return argSpec{name: name, desc: desc} }
func opt(name, desc string) argSpec { return argSpec{name: name, desc: desc, optional: true} }

func ok(script string) func(map[string]string) (string, error) {
	return func(map[string]string) (string, error) { return script, nil }
}

func one(f func(string) string, key string) func(map[string]string) (string, error) {
	return func(a map[string]string) (string, error) { return f(a[key]), nil }
}

func same(key string) func(map[string]string) string {
	return func(a map[string]string) string { return a[key] }
}

// kubeArgs adds the optional context and namespace to a kubectl call.
func kubeArgs(a map[string]string, rest ...string) string {
	var pre []string
	if a["context"] != "" {
		pre = append(pre, "--context", a["context"])
	}
	if a["namespace"] != "" {
		pre = append(pre, "-n", a["namespace"])
	}
	return shJoin("kubectl", append(pre, rest...)...)
}

var kubeOpts = []argSpec{opt("context", "kubectl context (default: the current one)"), opt("namespace", "namespace (default: the context's)")}

func kube(name, desc string, rest ...string) cmdSpec {
	return cmdSpec{name: name, desc: desc, args: kubeOpts, build: func(a map[string]string) (string, error) { return kubeArgs(a, rest...), nil }}
}

var argBucket, argKey = req("bucket", "bucket name"), req("key", "object key")
var queueURL = req("queue_url", "queue URL, from sqs.queues")
var topicARN = req("topic_arn", "topic ARN, from sns.topics")

var registry = []cmdSpec{
	{name: "aws.whoami", desc: "sts get-caller-identity: the account and user of the session", build: ok("aws sts get-caller-identity")},

	{name: "s3.list-buckets", desc: "buckets with creation date", build: ok(`aws s3api list-buckets --query 'Buckets[].[Name,CreationDate]' --output table`)},
	{name: "s3.browse", desc: "list a prefix of a bucket", args: []argSpec{argBucket, opt("prefix", "key prefix")},
		build: func(a map[string]string) (string, error) { return s3BrowseScript(a["bucket"], a["prefix"]), nil }},
	{name: "s3.audit", desc: "bucket exposure: versioning, public access block, policy, logging, object lock", args: []argSpec{argBucket}, build: one(s3AuditScript, "bucket")},
	{name: "s3.size", desc: "size and object count from CloudWatch", args: []argSpec{argBucket}, build: one(s3SizeScript, "bucket")},
	{name: "s3.head", desc: "object metadata", args: []argSpec{argBucket, argKey},
		build: func(a map[string]string) (string, error) { return s3HeadScript(a["bucket"], a["key"]), nil }},
	{name: "s3.versions", desc: "versions and delete markers of a key", args: []argSpec{argBucket, argKey},
		build: func(a map[string]string) (string, error) { return s3VersionsScript(a["bucket"], a["key"]), nil }},
	{name: "s3.preview", desc: "first 16 KB of an object, as text", args: []argSpec{argBucket, argKey},
		build: func(a map[string]string) (string, error) { return s3PreviewScript(a["bucket"], a["key"]), nil }},
	{name: "s3.presign", desc: "temporary download URL (the URL gives read access to whoever has it)", args: []argSpec{argBucket, argKey, opt("expires", "seconds, 1 to 604800, default 3600")},
		build: func(a map[string]string) (string, error) {
			n, err := parseExpires(a["expires"])
			if err != nil {
				return "", err
			}
			return s3PresignScript(a["bucket"], a["key"], n), nil
		}},

	{name: "ses.identities", desc: "verified domains and addresses", build: ok(`aws sesv2 list-email-identities --query 'EmailIdentities[].[IdentityName,IdentityType,SendingEnabled]' --output table`)},
	{name: "ses.identity", desc: "verification, DKIM and mail-from of one identity", args: []argSpec{req("identity", "domain or address")}, build: one(sesIdentityScript, "identity")},
	{name: "ses.suppressed-get", desc: "is this address on the suppression list, and why", args: []argSpec{req("address", "email address")}, build: one(sesSuppressedGetScript, "address")},
	{name: "ses.suppressed-list", desc: "the last 30 suppressed addresses", build: ok(sesSuppressedList)},
	{name: "ses.account", desc: "sandbox or production, quota, suppression settings", build: ok(sesAccountScript)},

	{name: "sns.topics", desc: "topic ARNs", build: ok(`aws sns list-topics --query 'Topics[].TopicArn' --output text`)},
	{name: "sns.subscriptions", desc: "subscriptions of a topic", args: []argSpec{topicARN}, build: one(snsSubsScript, "topic_arn")},
	{name: "sns.filters", desc: "filter and redrive policies of the subscriptions", args: []argSpec{topicARN}, build: one(snsFiltersScript, "topic_arn")},
	{name: "sns.attributes", desc: "attributes of a topic", args: []argSpec{topicARN}, build: one(snsAttrsScript, "topic_arn")},

	{name: "sqs.queues", desc: "queue URLs", build: ok(`aws sqs list-queues --query 'QueueUrls[]' --output text`)},
	{name: "sqs.attributes", desc: "all attributes, including the redrive policy and the message counts", args: []argSpec{queueURL}, build: one(sqsAttrsScript, "queue_url")},
	{name: "sqs.peek", desc: "up to 10 messages. It is a receive: the ReceiveCount of each message goes up", args: []argSpec{queueURL}, build: one(sqsPeekScript, "queue_url")},
	{name: "sqs.dlq-sources", desc: "queues that use this queue as a dead-letter queue", args: []argSpec{queueURL}, build: one(sqsDLQSourcesScript, "queue_url")},
	{name: "sqs.redrive-status", desc: "recent message move tasks of a DLQ", args: []argSpec{queueURL}, build: one(sqsRedriveStatusScript, "queue_url")},

	{name: "iam.my-access", desc: "groups and policies of the current user", build: ok(iamMyAccessScript)},
	{name: "iam.keys", desc: "access keys of the current user: status, age, last use", build: ok(iamKeysScript)},
	{name: "iam.mfa-devices", desc: "MFA devices of the current user", build: ok("aws iam list-mfa-devices --output table")},
	{name: "iam.roles", desc: "role names and ARNs", build: ok(`aws iam list-roles --query 'Roles[].[RoleName,Arn]' --output table`)},
	{name: "iam.role", desc: "trust policy and policies of a role", args: []argSpec{req("role", "role name")}, build: one(iamRoleScript, "role")},
	{name: "iam.simulate", desc: "can the current identity do this action? (MFA-aware when the profile is an -mfa one)",
		args: []argSpec{req("action", "for example s3:GetObject"), opt("resource", "resource ARN, default *")},
		build: func(a map[string]string) (string, error) {
			return iamSimulateScript(a["action"], a["resource"], a["_mfa"] == "true")
		}},

	kube("k8s.pods", "pods with status", "get", "pods"),
	kube("k8s.deployments", "deployments", "get", "deployments"),
	kube("k8s.events", "events sorted by time", "get", "events", "--sort-by=.lastTimestamp"),
	kube("k8s.top-pods", "CPU and memory of the pods", "top", "pods"),
	{name: "k8s.logs", desc: "the last lines of the logs of a pod", args: append([]argSpec{req("pod", "pod name"), opt("tail", "number of lines, default 200")}, kubeOpts...),
		build: func(a map[string]string) (string, error) {
			n := 200
			if a["tail"] != "" {
				v, err := strconv.Atoi(a["tail"])
				if err != nil || v < 1 || v > 5000 {
					return "", fmt.Errorf("tail must be a number from 1 to 5000")
				}
				n = v
			}
			return kubeArgs(a, "logs", "--tail="+strconv.Itoa(n), a["pod"]), nil
		}},

	// Commands below change data. They need --allow-write and the confirm value, like the TUI asks you to type the name.
	{name: "s3.undelete", desc: "WRITES: removes the latest delete marker of a key", write: true, confirm: same("key"), args: []argSpec{argBucket, argKey},
		build: func(a map[string]string) (string, error) { return s3UndeleteScript(a["bucket"], a["key"]), nil }},
	{name: "s3.upload", desc: "WRITES: copies a local file into the bucket", write: true, confirm: same("key"), args: []argSpec{argBucket, argKey, req("local", "local file path")},
		build: func(a map[string]string) (string, error) {
			return s3UploadScript(a["bucket"], a["key"], a["local"]), nil
		}},
	{name: "s3.download", desc: "WRITES a local file: copies an object to a folder", write: true, confirm: same("key"), args: []argSpec{argBucket, argKey, req("dest", "local folder")},
		build: func(a map[string]string) (string, error) {
			return s3DownloadScript(a["bucket"], a["key"], a["dest"]), nil
		}},
	{name: "ses.suppressed-delete", desc: "WRITES: removes an address from the suppression list", write: true, confirm: same("address"), args: []argSpec{req("address", "email address")}, build: one(sesSuppressedDeleteScript, "address")},
	{name: "ses.send", desc: "WRITES: sends a real email", write: true, confirm: same("to"), args: []argSpec{req("from", "verified sender"), req("to", "recipient"), req("subject", "subject"), req("body", "text body")},
		build: func(a map[string]string) (string, error) {
			return sesSendScript(a["from"], a["to"], a["subject"], a["body"])
		}},
	{name: "sns.publish", desc: "WRITES: publishes a message to every subscriber", write: true, confirm: func(a map[string]string) string { return after(":")(a["topic_arn"]) },
		args: []argSpec{topicARN, req("message", "message"), opt("subject", "subject")},
		build: func(a map[string]string) (string, error) {
			return snsPublishScript(a["topic_arn"], a["subject"], a["message"]), nil
		}},
	{name: "sqs.send", desc: "WRITES: adds a message to a queue", write: true, confirm: func(a map[string]string) string { return after("/")(a["queue_url"]) },
		args:  []argSpec{queueURL, req("body", "message body")},
		build: func(a map[string]string) (string, error) { return sqsSendScript(a["queue_url"], a["body"]), nil }},
	{name: "sqs.redrive", desc: "WRITES: moves every message of a DLQ back to the source queues", write: true, confirm: func(a map[string]string) string { return after("/")(a["queue_url"]) },
		args: []argSpec{queueURL}, build: one(sqsRedriveScript, "queue_url")},
	{name: "sqs.purge", desc: "WRITES: deletes every message of a queue. It cannot be undone", write: true, confirm: func(a map[string]string) string { return after("/")(a["queue_url"]) },
		args: []argSpec{queueURL}, build: one(sqsPurgeScript, "queue_url")},
}

func findCmd(name string) (cmdSpec, bool) {
	for _, c := range registry {
		if c.name == name {
			return c, true
		}
	}
	return cmdSpec{}, false
}

// needsConfirm explains what the caller must pass in "confirm".
func (c cmdSpec) needsConfirm() string {
	switch {
	case !c.write:
		return ""
	case c.name == "sqs.send" || c.name == "sqs.redrive" || c.name == "sqs.purge":
		return "the queue name (the last part of queue_url)"
	case c.name == "sns.publish":
		return "the topic name (the last part of topic_arn)"
	}
	return "the value of " + firstArg(c)
}

func firstArg(c cmdSpec) string {
	probe := map[string]string{}
	for _, a := range c.args {
		probe[a.name] = a.name
	}
	v := c.confirm(probe)
	return v
}

func sortedNames() []string {
	names := make([]string, 0, len(registry))
	for _, c := range registry {
		names = append(names, c.name)
	}
	sort.Strings(names)
	return names
}

func describe(c cmdSpec) string {
	var parts []string
	for _, a := range c.args {
		if a.optional {
			parts = append(parts, "["+a.name+"=]")
		} else {
			parts = append(parts, a.name+"=")
		}
	}
	return strings.TrimSpace(c.name + " " + strings.Join(parts, " "))
}
