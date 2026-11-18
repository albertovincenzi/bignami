package main

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	freshnessRe = regexp.MustCompile(`^\d{1,3}[smhd]$`)
	wordRe      = regexp.MustCompile(`^[a-z_]{1,20}$`)
)

var ghRepo = opt("repo", "owner/name (default: the repository of the current folder)")
var gcProject = opt("project", "Google Cloud project (default: the one of gcloud)")

func argOf(name string) func(map[string]string) string {
	return func(m map[string]string) string { return m[name] }
}

// limit reads a count that goes into a command.
func limit(v string, def, top int) (string, error) {
	if v == "" {
		return strconv.Itoa(def), nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > top {
		return "", fmt.Errorf("the number must be from 1 to %d", top)
	}
	return v, nil
}

func init() {
	registry = append(registry, []cmdSpec{
		// SES
		{name: "ses.reputation", desc: "account state and bounce and complaint rates of the last 7 days", build: ok(sesReputationScript)},
		{name: "ses.send-stats", desc: "send quota and sent, bounces, complaints per day", build: ok(sesStatsScript)},
		{name: "ses.suppressed-summary", desc: "suppressed addresses per reason", build: ok(sesSuppressedSummary)},
		{name: "ses.config-sets", desc: "configuration set names", build: ok(sesConfigSetsScript)},
		{name: "ses.config-set", desc: "a configuration set and its event destinations", args: []argSpec{req("name", "configuration set")}, build: one(sesConfigSetScript, "name")},
		{name: "ses.templates", desc: "email templates", build: ok(sesTemplatesScript)},
		{name: "ses.dkim-records", desc: "the DKIM CNAME records a domain must publish", args: []argSpec{req("identity", "domain")}, build: one(sesDkimScript, "identity")},

		// Kubernetes, beyond one namespace
		{name: "k8s.unhealthy", desc: "pods that are not running or not ready, every namespace", args: []argSpec{opt("context", "kubectl context")},
			build: func(a map[string]string) (string, error) { return k8sUnhealthy(a["context"]), nil }},
		{name: "k8s.restarts", desc: "the 20 pods with most restarts, every namespace", args: []argSpec{opt("context", "kubectl context")},
			build: func(a map[string]string) (string, error) { return k8sRestarts(a["context"]), nil }},
		{name: "k8s.warnings", desc: "the last 40 warning events, every namespace", args: []argSpec{opt("context", "kubectl context")},
			build: func(a map[string]string) (string, error) { return k8sWarnings(a["context"]), nil }},
		{name: "k8s.nodes", desc: "nodes with status, version and size", args: []argSpec{opt("context", "kubectl context")},
			build: func(a map[string]string) (string, error) { return kc(a["context"], "get", "nodes", "-o", "wide"), nil }},
		{name: "k8s.top-nodes", desc: "CPU and memory per node", args: []argSpec{opt("context", "kubectl context")},
			build: func(a map[string]string) (string, error) { return kc(a["context"], "top", "nodes"), nil }},

		// GitHub (the repository of the current folder, or repo=)
		{name: "gh.pr-list", desc: "open pull requests; search is a GitHub search such as author:@me or review-requested:@me", args: []argSpec{opt("search", "GitHub search"), ghRepo},
			build: func(a map[string]string) (string, error) {
				args := []string{"pr", "list", "--limit", "30"}
				if a["search"] != "" {
					args = append(args, "--search", a["search"])
				}
				return ghCmd(a["repo"], args...), nil
			}},
		{name: "gh.pr-view", desc: "a pull request with its reviews", args: []argSpec{req("number", "PR number"), ghRepo},
			build: func(a map[string]string) (string, error) {
				n, err := digits("number", a["number"])
				return ghCmd(a["repo"], "pr", "view", n), err
			}},
		{name: "gh.pr-checks", desc: "CI checks of a pull request", args: []argSpec{req("number", "PR number"), ghRepo},
			build: func(a map[string]string) (string, error) {
				n, err := digits("number", a["number"])
				return ghCmd(a["repo"], "pr", "checks", n), err
			}},
		{name: "gh.runs", desc: "recent workflow runs", args: []argSpec{opt("status", "for example failure, success, in_progress"), opt("limit", "default 20, max 100"), ghRepo},
			build: func(a map[string]string) (string, error) {
				n, err := limit(a["limit"], 20, 100)
				if err != nil {
					return "", err
				}
				args := []string{"run", "list", "--limit", n}
				if st := a["status"]; st != "" {
					if !wordRe.MatchString(st) {
						return "", fmt.Errorf("status must be a word such as failure")
					}
					args = append(args, "--status", st)
				}
				return ghCmd(a["repo"], args...), nil
			}},
		{name: "gh.run-failed-log", desc: "the failed steps of a run, the last 300 lines", args: []argSpec{req("run_id", "run id, from gh.runs"), ghRepo},
			build: func(a map[string]string) (string, error) {
				id, err := digits("run_id", a["run_id"])
				return ghRunFailedScript(a["repo"], id), err
			}},
		{name: "gh.releases", desc: "the last 10 releases", args: []argSpec{ghRepo},
			build: func(a map[string]string) (string, error) {
				return ghCmd(a["repo"], "release", "list", "--limit", "10"), nil
			}},
		{name: "gh.issues", desc: "open issues, assigned to you by default", args: []argSpec{opt("assignee", "login, default @me"), ghRepo},
			build: func(a map[string]string) (string, error) {
				who := a["assignee"]
				if who == "" {
					who = "@me"
				}
				return ghCmd(a["repo"], "issue", "list", "--assignee", who), nil
			}},
		{name: "gh.run-rerun", desc: "WRITES: starts the failed jobs of a run again", write: true, confirm: argOf("run_id"), args: []argSpec{req("run_id", "run id"), ghRepo},
			build: func(m map[string]string) (string, error) {
				id, err := digits("run_id", m["run_id"])
				return ghCmd(m["repo"], "run", "rerun", id, "--failed"), err
			}},

		// Helm
		{name: "helm.list", desc: "releases of a namespace, or of every namespace with all=true", args: append([]argSpec{opt("all", "true for every namespace")}, kubeOpts...),
			build: func(m map[string]string) (string, error) {
				if m["all"] == "true" {
					return helmCmd(m["context"], "", "list", "-A"), nil
				}
				return helmCmd(m["context"], m["namespace"], "list"), nil
			}},
		{name: "helm.failed", desc: "failed or pending releases, every namespace", args: []argSpec{opt("context", "kubectl context")},
			build: func(m map[string]string) (string, error) {
				return helmCmd(m["context"], "", "list", "-A", "--failed", "--pending"), nil
			}},
		{name: "helm.status", desc: "status of a release", args: append([]argSpec{req("release", "release name")}, kubeOpts...), build: helmOn("status")},
		{name: "helm.history", desc: "revisions of a release", args: append([]argSpec{req("release", "release name")}, kubeOpts...), build: helmOn("history")},
		{name: "helm.values", desc: "values you set on a release", args: append([]argSpec{req("release", "release name")}, kubeOpts...), build: helmOn("get", "values")},
		{name: "helm.rollback", desc: "WRITES: rolls a release back to a revision", write: true, confirm: argOf("release"),
			args: append([]argSpec{req("release", "release name"), req("revision", "revision number")}, kubeOpts...),
			build: func(m map[string]string) (string, error) {
				rev, err := digits("revision", m["revision"])
				return helmCmd(m["context"], m["namespace"], "rollback", m["release"], rev), err
			}},

		// Google Cloud (no command here prints a secret value)
		{name: "gcloud.config", desc: "the active account and project", build: ok("gcloud auth list && gcloud config list --format='text(core.account,core.project)'")},
		{name: "gcloud.clusters", desc: "GKE clusters", args: []argSpec{gcProject},
			build: func(m map[string]string) (string, error) {
				return gcCmd(m["project"], "container", "clusters", "list"), nil
			}},
		{name: "gcloud.secrets", desc: "secret names. Values are never read here", args: []argSpec{gcProject},
			build: func(m map[string]string) (string, error) { return gcCmd(m["project"], "secrets", "list"), nil }},
		{name: "gcloud.logs", desc: "Cloud Logging entries that match a filter", args: []argSpec{req("filter", "Cloud Logging filter"), opt("freshness", "for example 1h or 2d, default 1h"), opt("limit", "default 50, max 500"), gcProject},
			build: func(m map[string]string) (string, error) {
				fr := m["freshness"]
				if fr == "" {
					fr = "1h"
				}
				if !freshnessRe.MatchString(fr) {
					return "", fmt.Errorf("freshness must look like 30m, 1h or 2d")
				}
				n, err := limit(m["limit"], 50, 500)
				if err != nil {
					return "", err
				}
				return gcLogsScript(m["project"], m["filter"], fr, n), nil
			}},

		// Docker
		{name: "docker.ps", desc: "containers, with all=true also the stopped ones", args: []argSpec{opt("all", "true to include stopped")},
			build: func(m map[string]string) (string, error) {
				if m["all"] == "true" {
					return dockerCmd("ps", "-a"), nil
				}
				return dockerCmd("ps"), nil
			}},
		{name: "docker.logs", desc: "the last lines of a container's logs", args: []argSpec{req("name", "container"), opt("tail", "lines, default 200, max 5000")},
			build: func(m map[string]string) (string, error) {
				n, err := limit(m["tail"], 200, 5000)
				return dockerCmd("logs", "--tail", n, m["name"]) + " 2>&1", err
			}},
		{name: "docker.images", desc: "images", build: ok(dockerCmd("images"))},
		{name: "docker.df", desc: "disk used by images, containers, volumes and cache", build: ok(dockerCmd("system", "df"))},
		{name: "docker.restart", desc: "WRITES: restarts a container", write: true, confirm: argOf("name"), args: []argSpec{req("name", "container")},
			build: func(m map[string]string) (string, error) { return dockerCmd("restart", m["name"]), nil }},
	}...)
}

func helmOn(sub ...string) func(map[string]string) (string, error) {
	return func(m map[string]string) (string, error) {
		return helmCmd(m["context"], m["namespace"], append(sub, m["release"])...), nil
	}
}
