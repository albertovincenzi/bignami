package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albertovincenzi/bignami/internal/awsprofile"
	"github.com/albertovincenzi/bignami/internal/testutil"
)

// numericArgs are validated before they reach a command, so the tests give them valid values.
var numericArgs = map[string]string{"expires": "60", "tail": "10", "action": "s3:GetObject", "from": "a@b.co", "to": "c@d.co", "address": "c@d.co",
	"number": "12", "run_id": "99", "revision": "3", "limit": "10", "freshness": "2h", "status": "failure", "all": "true"}

func sampleArgs(c CmdSpec) map[string]string {
	over := map[string]string{}
	for k, v := range numericArgs {
		over[k] = v
	}
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
	for _, c := range Registry {
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
	testutil.StubAWS(t)
	purge, _ := FindCmd("sqs.purge")
	url := "https://sqs.eu-west-1.amazonaws.com/1/orders"
	if _, err := RunCmd(purge, map[string]string{"queue_url": url}, RunOpts{}); err == nil || !strings.Contains(err.Error(), "--allow-write") {
		t.Errorf("a write without --allow-write must fail: %v", err)
	}
	if _, err := RunCmd(purge, map[string]string{"queue_url": url}, RunOpts{allowWrite: true, confirm: "other"}); err == nil || !strings.Contains(err.Error(), `"orders"`) {
		t.Errorf("a wrong confirm must fail and name the expected value: %v", err)
	}
	if out, err := RunCmd(purge, map[string]string{"queue_url": url}, RunOpts{allowWrite: true, confirm: "orders"}); err != nil || !strings.Contains(out, "purge-queue") {
		t.Errorf("a confirmed write must run: %v %q", err, out)
	}
	audit, _ := FindCmd("s3.audit")
	if _, err := RunCmd(audit, nil, RunOpts{}); err == nil {
		t.Error("a missing required argument must fail")
	}
	if _, err := RunCmd(audit, map[string]string{"bucket": "b", "bogus": "x"}, RunOpts{}); err == nil {
		t.Error("an unknown argument must fail")
	}
}

func TestRunCmdNeverExecutesInput(t *testing.T) {
	testutil.StubAWS(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	for _, v := range []string{"x; touch " + marker, "$(touch " + marker + ")", "`touch " + marker + "`", "x\ntouch " + marker} {
		for _, c := range Registry {
			if c.write {
				continue
			}
			args := sampleArgs(c)
			for k := range args {
				if _, fixed := numericArgs[k]; !fixed {
					args[k] = v
				}
			}
			_, _ = RunCmd(c, args, RunOpts{})
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("%s ran the input %q", c.name, v)
			}
		}
	}
}

func TestRunCmdStopsAtTimeoutAndCapsOutput(t *testing.T) {
	start := time.Now()
	if _, err := ExecScript(os.Environ(), "sleep 30", 300*time.Millisecond); err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("a slow command must stop at the limit: %v after %s", err, time.Since(start))
	}
	text, err := ExecScript(os.Environ(), "yes x | head -c 400000", 5*time.Second)
	if err != nil || len(text) > MaxOutput+100 || !strings.Contains(text, "output cut") {
		t.Errorf("output must be capped: err=%v len=%d", err, len(text))
	}
}

func TestStatusReportsSession(t *testing.T) {
	testutil.StubAWS(t)
	i := Status("dev", "")
	if i.MFAValid || i.NextStep == "" || i.Profile != "dev" {
		t.Errorf("without a session the next step must say what to do: %+v", i)
	}
	exp := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	if err := awsprofile.WriteProfile("dev-mfa", [][2]string{{awsprofile.ExpirationKey, exp}}); err != nil {
		t.Fatal(err)
	}
	i = Status("dev", "")
	if !i.MFAValid || i.Profile != "dev-mfa" || i.RemainingSecs < 7000 || i.Region != "eu-west-1" {
		t.Errorf("a valid session must be preferred: %+v", i)
	}
	old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	_ = awsprofile.WriteProfile("dev-mfa", [][2]string{{awsprofile.ExpirationKey, old}})
	if i = Status("dev", ""); i.MFAValid || i.Profile != "dev" {
		t.Errorf("an expired session must not be used: %+v", i)
	}
}

func TestParseArgsAcceptsFlagsAnywhere(t *testing.T) {
	p, err := ParseArgs([]string{"s3.audit", "--profile", "dev", "bucket=my-b", "--region=eu-west-1", "--json", "--allow-write", "--confirm", "k"})
	if err != nil {
		t.Fatal(err)
	}
	if p.words[0] != "s3.audit" || p.kv["bucket"] != "my-b" || p.opts.profile != "dev" || p.opts.region != "eu-west-1" || !p.json || !p.opts.allowWrite || p.opts.confirm != "k" {
		t.Errorf("%+v", p)
	}
	if _, err := ParseArgs([]string{"--nope"}); err == nil {
		t.Error("an unknown flag must fail")
	}
	if _, err := ParseArgs([]string{"--profile"}); err == nil {
		t.Error("a flag without value must fail")
	}
}

func rpc(t *testing.T, o RunOpts, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := ServeMCP(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, o); err != nil {
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
	testutil.StubAWS(t)
	res := rpc(t, RunOpts{},
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
	for _, c := range Registry {
		if c.write && tools[ToolName(c.name)] != nil {
			t.Errorf("write tool %s must be hidden by default", c.name)
		}
	}
	if res[2]["error"] == nil {
		t.Error("an unknown method must return an error")
	}
}

func TestMCPWriteToolsNeedFlagAndConfirm(t *testing.T) {
	testutil.StubAWS(t)
	res := rpc(t, RunOpts{allowWrite: true},
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
	testutil.StubAWS(t)
	res := rpc(t, RunOpts{},
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
