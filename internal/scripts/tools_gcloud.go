package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

// GcCmd adds the project chosen in the TUI as a flag, so the global gcloud config is not changed.
func GcCmd(project string, args ...string) string {
	if project != "" {
		args = append(args, "--project", project)
	}
	return shell.Join("gcloud", args...)
}

func GcLogsScript(project, filter, freshness, limit string) string {
	return GcCmd(project, "logging", "read", filter, "--freshness", freshness, "--limit", limit, "--format", "value(timestamp,severity,resource.labels.pod_name,textPayload,jsonPayload.message)")
}
