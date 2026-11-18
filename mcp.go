package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// A small MCP server over stdio (newline-delimited JSON-RPC). It exposes the headless commands as tools.
// Read-only tools are always on. Writing tools exist only with --allow-write, and each one needs a "confirm" value.
const mcpVersion = "2025-06-18"

type rpcReq struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type mcpServer struct {
	out  io.Writer
	opts runOpts
}

func toolName(cmd string) string { return strings.NewReplacer(".", "_", "-", "_").Replace(cmd) }

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func schema(props map[string]any, required []string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func (s *mcpServer) tools() []tool {
	common := map[string]any{"profile": str("AWS profile (default: the one of the server)"), "region": str("AWS region (default: the profile's)")}
	ts := []tool{
		{"status", "The AWS profile, region, Kubernetes context and the MFA session of the user, with the time left. Call it first. If the session is expired, ask the user for the 6-digit MFA code and call login.",
			schema(common, nil)},
		{"login", "Starts a 12-hour MFA session. The user must give you the current 6-digit code from their authenticator.",
			schema(map[string]any{"code": str("6-digit MFA code"), "profile": common["profile"]}, []string{"code"})},
	}
	for _, c := range registry {
		if c.write && !s.opts.allowWrite {
			continue
		}
		props, required := map[string]any{}, []string{}
		for _, a := range c.args {
			props[a.name] = str(a.desc)
			if !a.optional {
				required = append(required, a.name)
			}
		}
		for k, v := range common {
			props[k] = v
		}
		desc := c.desc
		if !c.write {
			desc = "Read-only. " + desc
		} else {
			props["confirm"] = str("must be " + c.needsConfirm() + ". Ask the user before you send it")
			required = append(required, "confirm")
		}
		ts = append(ts, tool{toolName(c.name), desc, schema(props, required)})
	}
	return ts
}

func (s *mcpServer) call(name string, args map[string]string) (string, error) {
	o := s.opts
	if v := args["profile"]; v != "" {
		o.profile = v
	}
	if v := args["region"]; v != "" {
		o.region = v
	}
	switch name {
	case "status":
		b, _ := json.MarshalIndent(status(o.profile, o.region), "", "  ")
		return string(b), nil
	case "login":
		if err := loginCmd(o.profile, args["code"]); err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(status(o.profile, o.region), "", "  ")
		return string(b), nil
	}
	for _, c := range registry {
		if toolName(c.name) != name || (c.write && !s.opts.allowWrite) {
			continue
		}
		o.confirm = args["confirm"]
		rest := map[string]string{}
		for k, v := range args {
			if k != "profile" && k != "region" && k != "confirm" {
				rest[k] = v
			}
		}
		return runCmd(c, rest, o)
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *mcpServer) send(id json.RawMessage, result any, rpcErr map[string]any) {
	m := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id)}
	if rpcErr != nil {
		m["error"] = rpcErr
	} else {
		m["result"] = result
	}
	b, _ := json.Marshal(m)
	fmt.Fprintln(s.out, string(b))
}

func (s *mcpServer) handle(line []byte) {
	var r rpcReq
	if err := json.Unmarshal(line, &r); err != nil {
		s.send(json.RawMessage("null"), nil, map[string]any{"code": -32700, "message": "parse error"})
		return
	}
	if len(r.ID) == 0 { // a notification: no answer
		return
	}
	switch r.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(r.Params, &p)
		v := p.ProtocolVersion
		if v == "" {
			v = mcpVersion
		}
		s.send(r.ID, map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "bignami", "version": version},
			"instructions":    "AWS and Kubernetes helpers. Call status first. Tools are read-only unless the server runs with --allow-write.",
		}, nil)
	case "ping":
		s.send(r.ID, map[string]any{}, nil)
	case "tools/list":
		s.send(r.ID, map[string]any{"tools": s.tools()}, nil)
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(r.Params, &p); err != nil {
			s.send(r.ID, nil, map[string]any{"code": -32602, "message": "bad params"})
			return
		}
		args := map[string]string{}
		for k, v := range p.Arguments {
			args[k] = fmt.Sprint(v)
		}
		text, err := s.call(p.Name, args)
		if err != nil {
			text = strings.TrimSpace(text + "\nerror: " + err.Error())
		}
		s.send(r.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": err != nil}, nil)
	default:
		s.send(r.ID, nil, map[string]any{"code": -32601, "message": "method not found: " + r.Method})
	}
}

func serveMCP(in io.Reader, out io.Writer, o runOpts) error {
	s := &mcpServer{out: out, opts: o}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			s.handle([]byte(line))
		}
	}
	return sc.Err()
}
