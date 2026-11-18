package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAWS puts a stub `aws` first in PATH, so a script can run without touching any account.
func fakeAWS(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	stub := "#!/bin/sh\nexit 0\n" // silent: loops over its output (keys, subscriptions) then do nothing
	if err := os.WriteFile(filepath.Join(dir, "aws"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + dir + ":/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + t.TempDir()}
}

func TestScriptsAreValidShellAndNeverRunUserInput(t *testing.T) {
	env := fakeAWS(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	values := []string{"plain", "with space", `it's`, `a"b`,
		"x; touch " + marker, "$(touch " + marker + ")", "`touch " + marker + "`", "x\ntouch " + marker}
	for _, v := range values {
		scripts := map[string]string{
			"s3 browse":   S3BrowseScript("b", v),
			"s3 presign":  S3PresignScript("b", v, 60),
			"s3 head":     S3HeadScript("b", v),
			"s3 preview":  S3PreviewScript("b", v),
			"s3 versions": S3VersionsScript("b", v),
			"s3 undelete": S3UndeleteScript("b", v),
			"s3 download": S3DownloadScript("b", v, filepath.Join(t.TempDir(), "dest dir")),
			"s3 upload":   S3UploadScript("b", v, "/tmp/file name"),
			"s3 audit":    S3AuditScript(v),
			"s3 size":     S3SizeScript(v),
			"ses ident":   SesIdentityScript(v),
			"ses get":     SesSuppressedGetScript(v),
			"ses delete":  SesSuppressedDeleteScript(v),
			"sns subs":    SnsSubsScript(v),
			"sns filters": SnsFiltersScript(v),
			"sns attrs":   SnsAttrsScript(v),
			"sns publish": SnsPublishScript(v, v, v),
			"sqs attrs":   SqsAttrsScript(v),
			"sqs peek":    SqsPeekScript(v),
			"sqs send":    SqsSendScript(v, v),
			"sqs purge":   SqsPurgeScript(v),
			"sqs dlq":     SqsDLQSourcesScript(v),
			"sqs redrive": SqsRedriveScript(v),
			"sqs status":  SqsRedriveStatusScript(v),
			"iam role":    IamRoleScript(v),
			"iam keys":    IamKeysScript,
			"iam access":  IamMyAccessScript,
			"ses list":    SesSuppressedList,
			"ses account": SesAccountScript,
		}
		if s, err := SesSendScript("a@b.co", "c@d.co", v, v); err == nil {
			scripts["ses send"] = s
		} else {
			t.Errorf("ses send rejected valid addresses: %v", err)
		}
		if s, err := IamSimulateScript("s3:GetObject", v, true); err == nil {
			scripts["iam simulate"] = s
		}
		for name, script := range scripts {
			if out, err := exec.Command("sh", "-n", "-c", script).CombinedOutput(); err != nil {
				t.Errorf("%s with %q is not valid shell: %v\n%s\n%s", name, v, err, out, script)
				continue
			}
			cmd := exec.Command("sh", "-c", script)
			cmd.Env = env
			_ = cmd.Run() // the stub aws succeeds; other exit codes do not matter here, only the side effect below
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("%s ran the user input %q", name, v)
			}
		}
	}
}

func TestSesSendBuildsJSONAndValidates(t *testing.T) {
	s, err := SesSendScript("from@x.io", "to@y.io", `Sub "q" 'a'`, "line1\nline2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "--destination '{\"ToAddresses\":[\"to@y.io\"]}'") {
		t.Errorf("destination missing: %s", s)
	}
	i := strings.Index(s, "--content ") + len("--content ")
	raw := strings.ReplaceAll(strings.Trim(s[i:], "'"), `'\''`, "'")
	var content map[string]map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		t.Fatalf("content is not JSON: %v\n%s", err, raw)
	}
	if got := content["Simple"]["Subject"]["Data"]; got != `Sub "q" 'a'` {
		t.Errorf("subject = %v", got)
	}
	if _, err := SesSendScript("bad", "to@y.io", "s", "b"); err == nil {
		t.Error("an invalid sender must be rejected")
	}
}

func TestInputValidation(t *testing.T) {
	for in, ok := range map[string]bool{"": true, "60": true, "604800": true, "0": false, "604801": false, "abc": false, "-5": false} {
		if _, err := ParseExpires(in); (err == nil) != ok {
			t.Errorf("parseExpires(%q) ok=%v, want %v", in, err == nil, ok)
		}
	}
	for in, ok := range map[string]bool{"s3:GetObject": true, "sqs:*": true, "s3": false, "s3:Get Object": false, "": false} {
		if _, err := IamSimulateScript(in, "", false); (err == nil) != ok {
			t.Errorf("iamSimulateScript(%q) ok=%v, want %v", in, err == nil, ok)
		}
	}
}

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
	out, code := runWith(t, K8sInNS("ctx", "default", "get", "pods"), map[string]string{"kubectl": kubectl})
	if code != 0 || !strings.Contains(out, "No resources found") || !strings.Contains(out, "Namespaces that have pods") ||
		!strings.Contains(out, "2 kube-system") || !strings.Contains(out, "Switch namespace") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
	out, _ = runWith(t, K8sInNS("ctx", "app", "get", "pods"), map[string]string{"kubectl": `echo "NAME READY"; echo "web-1 1/1"`})
	if strings.Contains(out, "Namespaces that have pods") {
		t.Errorf("a full namespace needs no hint:\n%s", out)
	}
}

func TestUnhealthyPodsAndRestarts(t *testing.T) {
	stub := map[string]string{"kubectl": "cat <<'EOF'\n" + podTable + "\nEOF"}
	out, code := runWith(t, K8sUnhealthy("ctx"), stub)
	if code != 0 || !strings.Contains(out, "web-2") || !strings.Contains(out, "dns-1") || strings.Contains(out, "web-1") || strings.Contains(out, "nightly-1") {
		t.Errorf("unhealthy: exit %d\n%s", code, out)
	}
	healthy := map[string]string{"kubectl": "printf 'NAMESPACE NAME READY STATUS RESTARTS AGE\\napp web-1 1/1 Running 0 1d\\n'"}
	if out, _ = runWith(t, K8sUnhealthy("ctx"), healthy); !strings.Contains(out, "All pods are running and ready.") {
		t.Errorf("a healthy cluster must say so:\n%s", out)
	}
	out, _ = runWith(t, K8sRestarts("ctx"), stub)
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
	out, code := runWith(t, SesStatsScript, map[string]string{"aws": aws})
	if code != 0 || !strings.Contains(out, "2026-11-17") || !strings.Contains(out, "200") || !strings.Contains(out, "2.50%") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if strings.Index(out, "2026-11-16") > strings.Index(out, "2026-11-17") {
		t.Errorf("days must be sorted:\n%s", out)
	}
}

func TestOtherScriptsAreValidShell(t *testing.T) {
	scripts := map[string]string{
		"docker shell": DockerShellScript("web 'x'"),
		"gc logs":      GcLogsScript("p", `severity>="ERROR" AND x='y'`, "1h", "50"),
		"gh failed":    GhRunFailedScript("o/r", "12"),
		"helm":         HelmCmd("c", "n", "rollback", "r", "3"),
		"k8s ns":       K8sInNS("c'x", "n y", "get", "pods"),
		"k8s warn":     K8sWarnings("c"),
		"ses dkim":     SesDkimScript("a b.com"),
		"ses policies": SesPoliciesScript("x'y"),
		"ses cfg":      SesConfigSetScript("x'y"),
		"ses rep":      SesReputationScript,
		"ses summary":  SesSuppressedSummary,
	}
	for name, s := range scripts {
		if out, err := exec.Command("sh", "-n", "-c", s).CombinedOutput(); err != nil {
			t.Errorf("%s: %v %s", name, err, out)
		}
	}
}
