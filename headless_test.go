package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubAWS puts a silent `aws` first in PATH and gives the session a temporary credentials file.
func stubAWS(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aws"), []byte("#!/bin/sh\necho \"aws $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin:/usr/sbin:/sbin")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_REGION", "eu-west-1")
}

func sampleArgs(c cmdSpec) map[string]string {
	over := map[string]string{"expires": "60", "tail": "10", "action": "s3:GetObject", "from": "a@b.co", "to": "c@d.co", "address": "c@d.co"}
	a := map[string]string{}
	for _, s := range c.args {
		a[s.name] = "va'l ue"
		if v, ok := over[s.name]; ok {
			a[s.name] = v
		}
	}
	return a
}

func TestRegistryIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range registry {
		if seen[c.name] {
			t.Errorf("duplicate command %s", c.name)
		}
		seen[c.name] = true
		if c.write && c.confirm == nil {
			t.Errorf("%s writes but has no confirm value", c.name)
		}
		if c.write && !strings.Contains(c.desc, "WRITES") {
			t.Errorf("%s writes: the description must say WRITES", c.name)
		}
		script, err := c.build(sampleArgs(c))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if out, err := exec.Command("sh", "-n", "-c", script).CombinedOutput(); err != nil {
			t.Errorf("%s is not valid shell: %v %s", c.name, err, out)
		}
	}
}

func TestRunCmdChecksArgumentsAndWriteGate(t *testing.T) {
	stubAWS(t)
	purge, _ := findCmd("sqs.purge")
	url := "https://sqs.eu-west-1.amazonaws.com/1/orders"
	if _, err := runCmd(purge, map[string]string{"queue_url": url}, runOpts{}); err == nil || !strings.Contains(err.Error(), "--allow-write") {
		t.Errorf("a write without --allow-write must fail: %v", err)
	}
	if _, err := runCmd(purge, map[string]string{"queue_url": url}, runOpts{allowWrite: true, confirm: "other"}); err == nil || !strings.Contains(err.Error(), `"orders"`) {
		t.Errorf("a wrong confirm must fail and name the expected value: %v", err)
	}
	if out, err := runCmd(purge, map[string]string{"queue_url": url}, runOpts{allowWrite: true, confirm: "orders"}); err != nil || !strings.Contains(out, "purge-queue") {
		t.Errorf("a confirmed write must run: %v %q", err, out)
	}
	audit, _ := findCmd("s3.audit")
	if _, err := runCmd(audit, nil, runOpts{}); err == nil {
		t.Error("a missing required argument must fail")
	}
	if _, err := runCmd(audit, map[string]string{"bucket": "b", "bogus": "x"}, runOpts{}); err == nil {
		t.Error("an unknown argument must fail")
	}
}

func TestRunCmdNeverExecutesInput(t *testing.T) {
	stubAWS(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	for _, v := range []string{"x; touch " + marker, "$(touch " + marker + ")", "`touch " + marker + "`", "x\ntouch " + marker} {
		for _, c := range registry {
			if c.write {
				continue
			}
			args := sampleArgs(c)
			for k := range args {
				if k != "expires" && k != "tail" && k != "action" {
					args[k] = v
				}
			}
			_, _ = runCmd(c, args, runOpts{})
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("%s ran the input %q", c.name, v)
			}
		}
	}
}

func TestRunCmdStopsAtTimeoutAndCapsOutput(t *testing.T) {
	start := time.Now()
	if _, err := execScript(os.Environ(), "sleep 30", 300*time.Millisecond); err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("a slow command must stop at the limit: %v after %s", err, time.Since(start))
	}
	text, err := execScript(os.Environ(), "yes x | head -c 400000", 5*time.Second)
	if err != nil || len(text) > maxOutput+100 || !strings.Contains(text, "output cut") {
		t.Errorf("output must be capped: err=%v len=%d", err, len(text))
	}
}

func TestStatusReportsSession(t *testing.T) {
	stubAWS(t)
	i := status("dev", "")
	if i.MFAValid || i.NextStep == "" || i.Profile != "dev" {
		t.Errorf("without a session the next step must say what to do: %+v", i)
	}
	exp := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	if err := writeProfile("dev-mfa", [][2]string{{expirationKey, exp}}); err != nil {
		t.Fatal(err)
	}
	i = status("dev", "")
	if !i.MFAValid || i.Profile != "dev-mfa" || i.RemainingSecs < 7000 || i.Region != "eu-west-1" {
		t.Errorf("a valid session must be preferred: %+v", i)
	}
	old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	_ = writeProfile("dev-mfa", [][2]string{{expirationKey, old}})
	if i = status("dev", ""); i.MFAValid || i.Profile != "dev" {
		t.Errorf("an expired session must not be used: %+v", i)
	}
}

func TestParseArgsAcceptsFlagsAnywhere(t *testing.T) {
	p, err := parseArgs([]string{"s3.audit", "--profile", "dev", "bucket=my-b", "--region=eu-west-1", "--json", "--allow-write", "--confirm", "k"})
	if err != nil {
		t.Fatal(err)
	}
	if p.words[0] != "s3.audit" || p.kv["bucket"] != "my-b" || p.opts.profile != "dev" || p.opts.region != "eu-west-1" || !p.json || !p.opts.allowWrite || p.opts.confirm != "k" {
		t.Errorf("%+v", p)
	}
	if _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Error("an unknown flag must fail")
	}
	if _, err := parseArgs([]string{"--profile"}); err == nil {
		t.Error("a flag without value must fail")
	}
}

func rpc(t *testing.T, o runOpts, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, o); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		res = append(res, m)
	}
	return res
}

func toolNames(m map[string]any) map[string]map[string]any {
	names := map[string]map[string]any{}
	for _, x := range m["result"].(map[string]any)["tools"].([]any) {
		tl := x.(map[string]any)
		names[tl["name"].(string)] = tl
	}
	return names
}

func TestMCPServerListsOnlyReadToolsByDefault(t *testing.T) {
	stubAWS(t)
	res := rpc(t, runOpts{},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"nope"}`)
	if len(res) != 3 {
		t.Fatalf("a notification must get no answer: %d answers", len(res))
	}
	if res[0]["result"].(map[string]any)["protocolVersion"] != "2025-03-26" {
		t.Error("the server must echo the protocol version")
	}
	tools := toolNames(res[1])
	for _, want := range []string{"status", "login", "s3_audit", "sqs_peek", "k8s_pods"} {
		if tools[want] == nil {
			t.Errorf("missing tool %s", want)
		}
	}
	for _, c := range registry {
		if c.write && tools[toolName(c.name)] != nil {
			t.Errorf("write tool %s must be hidden by default", c.name)
		}
	}
	if res[2]["error"] == nil {
		t.Error("an unknown method must return an error")
	}
}

func TestMCPWriteToolsNeedFlagAndConfirm(t *testing.T) {
	stubAWS(t)
	res := rpc(t, runOpts{allowWrite: true},
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sqs_purge","arguments":{"queue_url":"https://x/1/orders"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sqs_purge","arguments":{"queue_url":"https://x/1/orders","confirm":"orders"}}}`)
	purge := toolNames(res[0])["sqs_purge"]
	if purge == nil || !strings.Contains(purge["description"].(string), "WRITES") {
		t.Fatal("with --allow-write the write tool must exist and say WRITES")
	}
	if !strings.Contains(string(mustJSON(purge["inputSchema"])), `"confirm"`) {
		t.Error("a write tool must require confirm")
	}
	if res[1]["result"].(map[string]any)["isError"] != true {
		t.Error("a write without confirm must be an error")
	}
	if res[2]["result"].(map[string]any)["isError"] != false {
		t.Errorf("a confirmed write must succeed: %v", res[2])
	}
}

func TestMCPCallRunsReadToolAndHidesWriteToolByDefault(t *testing.T) {
	stubAWS(t)
	res := rpc(t, runOpts{},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"s3_audit","arguments":{"bucket":"my-b"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sqs_purge","arguments":{"queue_url":"https://x/1/q","confirm":"q"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"status","arguments":{}}}`)
	text := func(i int) string {
		return res[i]["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	}
	if !strings.Contains(text(0), "get-bucket-versioning") || res[0]["result"].(map[string]any)["isError"] != false {
		t.Errorf("s3_audit must run: %q", text(0))
	}
	if res[1]["result"].(map[string]any)["isError"] != true || !strings.Contains(text(1), "unknown tool") {
		t.Errorf("a write tool must be unknown without --allow-write: %q", text(1))
	}
	if !strings.Contains(text(2), `"next_step"`) {
		t.Errorf("status must tell what to do: %q", text(2))
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
