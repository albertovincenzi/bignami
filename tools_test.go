package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWith runs a script with fake commands first in PATH. Each stub is the body of a shell script.
func runWith(t *testing.T, script string, stubs map[string]string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin"}
	b, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return string(b), code
}

const podTable = `NAMESPACE NAME READY STATUS RESTARTS AGE
app web-1 1/1 Running 0 1d
app web-2 0/1 CrashLoopBackOff 12 1d
jobs nightly-1 0/1 Completed 0 2h
kube-system dns-1 1/2 Running 3 9d`

func TestEmptyNamespaceListsNamespacesWithPods(t *testing.T) {
	kubectl := `case "$*" in *custom-columns*) printf 'kube-system\nkube-system\napp\n';; *) echo "No resources found in default namespace.";; esac`
	out, code := runWith(t, k8sInNS("ctx", "default", "get", "pods"), map[string]string{"kubectl": kubectl})
	if code != 0 || !strings.Contains(out, "No resources found") || !strings.Contains(out, "Namespaces that have pods") ||
		!strings.Contains(out, "2 kube-system") || !strings.Contains(out, "Switch namespace") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
	out, _ = runWith(t, k8sInNS("ctx", "app", "get", "pods"), map[string]string{"kubectl": `echo "NAME READY"; echo "web-1 1/1"`})
	if strings.Contains(out, "Namespaces that have pods") {
		t.Errorf("a full namespace needs no hint:\n%s", out)
	}
}

func TestUnhealthyPodsAndRestarts(t *testing.T) {
	stub := map[string]string{"kubectl": "cat <<'EOF'\n" + podTable + "\nEOF"}
	out, code := runWith(t, k8sUnhealthy("ctx"), stub)
	if code != 0 || !strings.Contains(out, "web-2") || !strings.Contains(out, "dns-1") || strings.Contains(out, "web-1") || strings.Contains(out, "nightly-1") {
		t.Errorf("unhealthy: exit %d\n%s", code, out)
	}
	healthy := map[string]string{"kubectl": "printf 'NAMESPACE NAME READY STATUS RESTARTS AGE\\napp web-1 1/1 Running 0 1d\\n'"}
	if out, _ = runWith(t, k8sUnhealthy("ctx"), healthy); !strings.Contains(out, "All pods are running and ready.") {
		t.Errorf("a healthy cluster must say so:\n%s", out)
	}
	out, _ = runWith(t, k8sRestarts("ctx"), stub)
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[0], "NAMESPACE") || !strings.Contains(lines[1], "web-2") {
		t.Errorf("most restarts first, header kept:\n%s", out)
	}
}

func TestSesStatsSumsPerDay(t *testing.T) {
	aws := `case "$*" in
*get-send-quota*) echo '{"Max24HourSend": 200}';;
*get-send-statistics*) printf '2026-11-17T10:00:00Z\t100\t2\t0\t0\n2026-11-17T10:15:00Z\t100\t3\t1\t0\n2026-11-16T10:00:00Z\t50\t0\t0\t0\n';;
esac`
	out, code := runWith(t, sesStatsScript, map[string]string{"aws": aws})
	if code != 0 || !strings.Contains(out, "2026-11-17") || !strings.Contains(out, "200") || !strings.Contains(out, "2.50%") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if strings.Index(out, "2026-11-16") > strings.Index(out, "2026-11-17") {
		t.Errorf("days must be sorted:\n%s", out)
	}
}

func TestOtherScriptsAreValidShell(t *testing.T) {
	scripts := map[string]string{
		"docker shell": dockerShellScript("web 'x'"),
		"gc logs":      gcLogsScript("p", `severity>="ERROR" AND x='y'`, "1h", "50"),
		"gh failed":    ghRunFailedScript("o/r", "12"),
		"helm":         helmCmd("c", "n", "rollback", "r", "3"),
		"k8s ns":       k8sInNS("c'x", "n y", "get", "pods"),
		"k8s warn":     k8sWarnings("c"),
		"ses dkim":     sesDkimScript("a b.com"),
		"ses policies": sesPoliciesScript("x'y"),
		"ses cfg":      sesConfigSetScript("x'y"),
		"ses rep":      sesReputationScript,
		"ses summary":  sesSuppressedSummary,
	}
	for name, s := range scripts {
		if out, err := exec.Command("sh", "-n", "-c", s).CombinedOutput(); err != nil {
			t.Errorf("%s: %v %s", name, err, out)
		}
	}
}

func TestEveryDomainHasProductsWithCommands(t *testing.T) {
	want := map[string]bool{"AWS": true, "Kubernetes": true, "GitHub": true, "Helm": true, "Google Cloud": true, "Docker": true}
	for _, d := range domains {
		delete(want, d.name)
		if d.icon == "" || d.color == nil {
			t.Errorf("%s needs an icon and a color", d.name)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing domains: %v", want)
	}
}
