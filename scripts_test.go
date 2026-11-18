package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
			"s3 browse":   s3BrowseScript("b", v),
			"s3 presign":  s3PresignScript("b", v, 60),
			"s3 head":     s3HeadScript("b", v),
			"s3 preview":  s3PreviewScript("b", v),
			"s3 versions": s3VersionsScript("b", v),
			"s3 undelete": s3UndeleteScript("b", v),
			"s3 download": s3DownloadScript("b", v, filepath.Join(t.TempDir(), "dest dir")),
			"s3 upload":   s3UploadScript("b", v, "/tmp/file name"),
			"s3 audit":    s3AuditScript(v),
			"s3 size":     s3SizeScript(v),
			"ses ident":   sesIdentityScript(v),
			"ses get":     sesSuppressedGetScript(v),
			"ses delete":  sesSuppressedDeleteScript(v),
			"sns subs":    snsSubsScript(v),
			"sns filters": snsFiltersScript(v),
			"sns attrs":   snsAttrsScript(v),
			"sns publish": snsPublishScript(v, v, v),
			"sqs attrs":   sqsAttrsScript(v),
			"sqs peek":    sqsPeekScript(v),
			"sqs send":    sqsSendScript(v, v),
			"sqs purge":   sqsPurgeScript(v),
			"sqs dlq":     sqsDLQSourcesScript(v),
			"sqs redrive": sqsRedriveScript(v),
			"sqs status":  sqsRedriveStatusScript(v),
			"iam role":    iamRoleScript(v),
			"iam keys":    iamKeysScript,
			"iam access":  iamMyAccessScript,
			"ses list":    sesSuppressedList,
			"ses account": sesAccountScript,
		}
		if s, err := sesSendScript("a@b.co", "c@d.co", v, v); err == nil {
			scripts["ses send"] = s
		} else {
			t.Errorf("ses send rejected valid addresses: %v", err)
		}
		if s, err := iamSimulateScript("s3:GetObject", v, true); err == nil {
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
	s, err := sesSendScript("from@x.io", "to@y.io", `Sub "q" 'a'`, "line1\nline2")
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
	if _, err := sesSendScript("bad", "to@y.io", "s", "b"); err == nil {
		t.Error("an invalid sender must be rejected")
	}
}

func TestInputValidation(t *testing.T) {
	for in, ok := range map[string]bool{"": true, "60": true, "604800": true, "0": false, "604801": false, "abc": false, "-5": false} {
		if _, err := parseExpires(in); (err == nil) != ok {
			t.Errorf("parseExpires(%q) ok=%v, want %v", in, err == nil, ok)
		}
	}
	for in, ok := range map[string]bool{"s3:GetObject": true, "sqs:*": true, "s3": false, "s3:Get Object": false, "": false} {
		if _, err := iamSimulateScript(in, "", false); (err == nil) != ok {
			t.Errorf("iamSimulateScript(%q) ok=%v, want %v", in, err == nil, ok)
		}
	}
	for in, ok := range map[string]bool{"eu-west-1": true, "us-gov-west-1": true, "europe": false, "EU-WEST-1": false} {
		if regionRe.MatchString(in) != ok {
			t.Errorf("regionRe(%q) = %v", in, !ok)
		}
	}
}

func TestAwsEnvSetsProfileAndRegion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "static")
	t.Setenv("AWS_REGION", "us-east-1")
	env := strings.Join(awsEnv("dev", "eu-west-1"), "\n")
	for _, want := range []string{"AWS_PROFILE=dev", "AWS_REGION=eu-west-1", "AWS_DEFAULT_REGION=eu-west-1"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(env, "AWS_ACCESS_KEY_ID") || strings.Contains(env, "us-east-1") {
		t.Error("static credentials and the old region must be dropped")
	}
	if strings.Contains(strings.Join(awsEnv("dev", ""), "\n"), "AWS_REGION") {
		t.Error("with no region the profile default must apply")
	}
}

func TestRecentsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	r := loadRecents()
	r.set("Bucket", "my-bucket")
	if got := loadRecents().get("Bucket"); got != "my-bucket" {
		t.Errorf("got %q after reload", got)
	}
	if st, err := os.Stat(r.path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("recent file: %v %v", err, st)
	}
	var nilRecents *recents
	nilRecents.set("x", "y") // must not panic
	if nilRecents.get("x") != "" {
		t.Error("nil recents must return empty")
	}
	if !recallable("Object key", false) || recallable("MFA code", true) || recallable("aws", false) || recallable("kubectl (context x)", false) || recallable("x - type 'x' to confirm", false) {
		t.Error("recallable rules are wrong")
	}
}

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeLine(m *model, s string) {
	m.Update(key(s))
	m.Update(key("enter"))
}

func TestAskCollectsValuesInOrderAndCancelsOnEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel()
	m.w, m.h = 120, 40
	var got []string
	m.ask([]promptSpec{{label: "A"}, {label: "B", optional: true}, {label: "C"}}, func(m *model, v []string) tea.Cmd {
		got = v
		return nil
	})
	typeLine(m, "one")
	m.Update(key("enter")) // B left empty
	typeLine(m, "three")
	if strings.Join(got, "|") != "one||three" {
		t.Errorf("got %q", got)
	}

	got = nil
	m.ask([]promptSpec{{label: "A2"}}, func(*model, []string) tea.Cmd { got = []string{"ran"}; return nil })
	m.Update(key("enter"))
	if got != nil || !strings.Contains(m.err, "required") {
		t.Errorf("an empty required value must cancel: got=%v err=%q", got, m.err)
	}
}

func TestConfirmNameRunsOnlyOnExactMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel()
	m.w, m.h = 120, 40
	ran := 0
	next := func(*model) tea.Cmd { ran++; return nil }
	m.confirmName("This deletes", "queue-a", next)
	typeLine(m, "queue-b")
	if ran != 0 || m.err != "cancelled" {
		t.Errorf("wrong name must cancel: ran=%d err=%q", ran, m.err)
	}
	m.confirmName("This deletes", "queue-a", next)
	typeLine(m, "queue-a")
	if ran != 1 {
		t.Errorf("exact name must run once, ran=%d", ran)
	}
}

func TestConfirmPromptsAreNeverPrefilled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel()
	m.recents.set("x - type 'x' to confirm", "x")
	m.confirmName("x", "x", func(*model) tea.Cmd { return nil })
	if m.in.Value() != "" {
		t.Errorf("a confirmation must start empty, got %q", m.in.Value())
	}
}

func TestEveryProductHasRunnableItems(t *testing.T) {
	for _, d := range domains {
		if len(d.products) == 0 {
			t.Errorf("%s has no products", d.name)
		}
		for _, p := range d.products {
			items := p.items()
			if len(items) == 0 {
				t.Errorf("%s/%s is empty", d.name, p.name)
			}
			for _, it := range items {
				i := it.(item)
				if i.title == "" || i.act == nil {
					t.Errorf("%s/%s has an incomplete item: %+v", d.name, p.name, i.title)
				}
			}
		}
	}
}

func TestProductScreensFitTheWindow(t *testing.T) {
	t.Setenv("HOME", filepath.Join(t.TempDir()))
	for _, size := range [][2]int{{70, 24}, {110, 30}, {130, 40}, {200, 50}} {
		for di, d := range domains {
			for _, p := range d.products {
				m := newModel()
				m.w, m.h = size[0], size[1]
				m.sel = di
				m.openDomain()
				m.push(p.name, p.items())
				v := m.View()
				for i, ln := range strings.Split(v, "\n") {
					if w := lineWidth(ln); w > m.w {
						t.Errorf("%s/%s %v line %d is %d wide", d.name, p.name, size, i, w)
					}
				}
				if n := strings.Count(v, "\n") + 1; n > m.h {
					t.Errorf("%s/%s %v has %d lines, window is %d", d.name, p.name, size, n, m.h)
				}
			}
		}
	}
}
