package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var iamActionRe = regexp.MustCompile(`^[A-Za-z0-9-]+:[A-Za-z0-9*]+$`)

var iamProduct = product{name: "IAM", desc: "access, permission check, keys, roles", icon: "◍", color: cAWS, items: iamItems}

func iamItems() []list.Item {
	a := acmd("◍")
	return []list.Item{
		a("My access", "groups, attached and inline policies", do(iamMyAccessScript)),
		a("Can I do this?", "simulates an action, MFA-aware", func(m *model) tea.Cmd {
			mfa := strings.HasSuffix(m.awsProfile, mfaSuffix)
			return m.ask([]promptSpec{{label: "Action, e.g. s3:GetObject"}, {label: "Resource ARN (default *)", optional: true}}, func(m *model, v []string) tea.Cmd {
				script, err := iamSimulateScript(v[0], v[1], mfa)
				if err != nil {
					m.err = err.Error()
					return nil
				}
				return m.sh(script)
			})
		}),
		a("My access keys", "status, age and last use", do(iamKeysScript)),
		a("My MFA devices", "serial numbers", do("aws iam list-mfa-devices --output table")),
		a("Roles", "trust policy and policies", pickAWS("roles",
			[]string{"iam", "list-roles", "--query", "Roles[].RoleName", "--output", "text"}, "◍", nil, iamRole)),
	}
}

func iamRole(m *model, role string) tea.Cmd {
	a := acmd("▸")
	m.push(role, []list.Item{
		a("Details", "trust policy and attached policies", do(iamRoleScript(role))),
	})
	return nil
}

const iamMyAccessScript = `U=$(aws iam get-user --query User.UserName --output text) || exit 1
echo "== user: $U"
echo "== groups"; aws iam list-groups-for-user --user-name "$U" --query 'Groups[].GroupName' --output text
echo "== attached policies"; aws iam list-attached-user-policies --user-name "$U" --query 'AttachedPolicies[].PolicyName' --output text
echo "== inline policies"; aws iam list-user-policies --user-name "$U" --query PolicyNames --output text
for G in $(aws iam list-groups-for-user --user-name "$U" --query 'Groups[].GroupName' --output text); do
  echo "== policies of group $G"; aws iam list-attached-group-policies --group-name "$G" --query 'AttachedPolicies[].PolicyName' --output text
done`

const iamKeysScript = `U=$(aws iam get-user --query User.UserName --output text) || exit 1
for K in $(aws iam list-access-keys --user-name "$U" --query 'AccessKeyMetadata[].AccessKeyId' --output text); do
  echo "== $K"
  aws iam list-access-keys --user-name "$U" --query "AccessKeyMetadata[?AccessKeyId=='$K'].[Status,CreateDate]" --output text | while read -r ST CD; do
    D=$(python3 -c 'import sys,datetime as d; t=d.datetime.fromisoformat(sys.argv[1].replace("Z","+00:00")); print((d.datetime.now(d.timezone.utc)-t).days)' "$CD" 2>/dev/null)
    echo "status $ST, created $CD${D:+ ($D days old)}"
    if [ "${D:-0}" -gt 90 ]; then echo "  warning: older than 90 days, rotate it"; fi
  done
  aws iam get-access-key-last-used --access-key-id "$K" --query 'AccessKeyLastUsed.[LastUsedDate,ServiceName,Region]' --output text
done`

func iamSimulateScript(action, resource string, mfa bool) (string, error) {
	if !iamActionRe.MatchString(action) {
		return "", fmt.Errorf("the action must look like service:Action, for example s3:GetObject")
	}
	if resource == "" {
		resource = "*"
	}
	ctx := "ContextKeyName=aws:MultiFactorAuthPresent,ContextKeyValues=" + strconv.FormatBool(mfa) + ",ContextKeyType=boolean"
	return `ARN=$(aws sts get-caller-identity --query Arn --output text) && aws iam simulate-principal-policy --policy-source-arn "$ARN" --action-names ` +
		shQuote(action) + ` --resource-arns ` + shQuote(resource) + ` --context-entries ` + shQuote(ctx) +
		` --query 'EvaluationResults[].[EvalActionName,EvalDecision,EvalResourceName]' --output table`, nil
}

func iamRoleScript(role string) string {
	return `R=` + shQuote(role) + `
aws iam get-role --role-name "$R" --query 'Role.[Arn,Description,MaxSessionDuration]' --output text
echo "== trust policy"; aws iam get-role --role-name "$R" --query Role.AssumeRolePolicyDocument
echo "== attached policies"; aws iam list-attached-role-policies --role-name "$R" --query 'AttachedPolicies[].PolicyName' --output text
echo "== inline policies"; aws iam list-role-policies --role-name "$R" --query PolicyNames --output text`
}
