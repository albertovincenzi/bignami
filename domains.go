package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type domain struct {
	name  string
	items func() []list.Item
}

var domains = []domain{
	{name: "AWS", items: awsItems},
	{name: "Kubernetes", items: k8sItems},
}

func aws(title, desc string, act func(m *model) tea.Cmd) item {
	return item{title: title, desc: desc, icon: iconAWS, color: cAWS, act: act}
}

func k8s(title, desc string, act func(m *model) tea.Cmd) item {
	return item{title: title, desc: desc, icon: iconK8s, color: cK8s, act: act}
}

// awsRun runs a script with the selected profile and waits for Enter.
func awsRun(script string) func(m *model) tea.Cmd {
	return func(m *model) tea.Cmd { return m.capture(awsEnv(m.awsProfile), script) }
}

// awsAsk asks for one value, then runs build(value). The value is shell-quoted.
func awsAsk(label string, build func(q string) string) func(m *model) tea.Cmd {
	return func(m *model) tea.Cmd {
		return m.prompt(label, false, func(m *model, v string) tea.Cmd {
			if v == "" {
				return nil
			}
			return m.capture(awsEnv(m.awsProfile), build(shQuote(v)))
		})
	}
}

// bucketAudit prints the security-relevant settings of a bucket. Common checks for exposure and retention.
const bucketAudit = `B=@B@; pg() { printf '\n== %s ==\n' "$1"; shift; "$@" 2>&1 | head -40; }
pg head-bucket aws s3api head-bucket --bucket "$B"
pg versioning aws s3api get-bucket-versioning --bucket "$B"
pg public-access-block aws s3api get-public-access-block --bucket "$B"
pg policy aws s3api get-bucket-policy --bucket "$B" --query Policy --output text
pg logging aws s3api get-bucket-logging --bucket "$B"
pg object-lock aws s3api get-object-lock-configuration --bucket "$B"`

func awsItems() []list.Item {
	return []list.Item{
		aws("MFA login", "12h session, saved as profile <name>-mfa", mfaLogin),
		aws("Who am I", "sts get-caller-identity", awsRun("aws sts get-caller-identity")),
		aws("Switch profile", "choose from ~/.aws/config", switchProfile),
		aws("Shell with this profile", "subshell with AWS_PROFILE set; exit to return", func(m *model) tea.Cmd {
			return runSh(awsEnv(m.awsProfile), `exec "${SHELL:-sh}"`, false)
		}),
		aws("My MFA devices and keys", "iam list-mfa-devices, list-access-keys", awsRun(
			`aws iam list-mfa-devices --output table; aws iam list-access-keys --output table`)),
		aws("S3: list buckets", "name and creation date", awsRun(
			`aws s3api list-buckets --query 'Buckets[].[Name,CreationDate]' --output table`)),
		aws("S3: browse a bucket", "top level of s3://<bucket>[/prefix]", awsAsk("Bucket (and optional prefix)",
			func(q string) string { return `aws s3 ls "s3://"` + q + ` --human-readable | ${PAGER:-less -R}` })),
		aws("S3: audit a bucket", "versioning, public access, policy, logging, object lock", awsAsk("Bucket name",
			func(q string) string {
				return "{ " + strings.Replace(bucketAudit, "@B@", q, 1) + "\n} | ${PAGER:-less -R}"
			})),
		aws("Logs: tail a log group", "aws logs tail --follow, last 10 minutes", awsAsk("CloudWatch log group",
			func(q string) string { return `aws logs tail ` + q + ` --follow --since 10m` })),
		aws("Decode an access error", "sts decode-authorization-message", awsAsk("Encoded message from the error",
			func(q string) string {
				return `aws sts decode-authorization-message --encoded-message ` + q + ` --query DecodedMessage --output text | python3 -m json.tool | ${PAGER:-less -R}`
			})),
		aws("Custom aws command", "aws <args>", func(m *model) tea.Cmd {
			return m.prompt("aws", false, func(m *model, v string) tea.Cmd {
				if v == "" {
					return nil
				}
				return m.capture(awsEnv(m.awsProfile), "aws "+v)
			})
		}),
	}
}

func mfaLogin(m *model) tea.Cmd {
	base := strings.TrimSuffix(m.awsProfile, mfaSuffix)
	return m.prompt("MFA code for profile "+base, true, func(m *model, code string) tea.Cmd {
		m.busy = "logging in..."
		return func() tea.Msg {
			if err := awsLogin(base, code); err != nil {
				return errMsg{err}
			}
			return loginDoneMsg{base + mfaSuffix}
		}
	})
}

func switchProfile(m *model) tea.Cmd {
	return m.load("profiles", func() ([]list.Item, error) {
		ps, err := awsProfiles()
		if err != nil {
			return nil, err
		}
		var items []list.Item
		for _, p := range ps {
			items = append(items, aws(p, "", func(m *model) tea.Cmd {
				m.awsProfile = p
				return m.home("AWS profile: " + p)
			}))
		}
		return items, nil
	})
}

func k8sItems() []list.Item {
	view := func(args ...string) func(m *model) tea.Cmd {
		return func(m *model) tea.Cmd {
			return m.capture(nil, shJoin("kubectl", m.kargs(args...)...))
		}
	}
	return []list.Item{
		k8s("Switch context", "only for this TUI, kubeconfig is not changed", func(m *model) tea.Cmd { return m.k8sContexts() }),
		k8s("Switch namespace", "", func(m *model) tea.Cmd { return m.k8sNamespaces() }),
		k8s("Pods", "logs, shell, describe", func(m *model) tea.Cmd { return m.k8sPods() }),
		k8s("Deployments", "get deployments", view("get", "deployments")),
		k8s("Recent events", "sorted by time", view("get", "events", "--sort-by=.lastTimestamp")),
		k8s("Top pods", "CPU and memory", view("top", "pods")),
		k8s("Port-forward", "e.g. svc/name 8080:80", func(m *model) tea.Cmd {
			return m.prompt("kubectl port-forward (context "+m.kubeCtx+")", false, func(m *model, v string) tea.Cmd {
				if v == "" {
					return nil
				}
				return m.guardProd(func(m *model) tea.Cmd {
					return m.capture(nil, shJoin("kubectl", m.kargs()...)+" port-forward "+v)
				})
			})
		}),
		k8s("Custom kubectl command", "kubectl --context <ctx> <args>", func(m *model) tea.Cmd { return m.kubectlPrompt() }),
	}
}
