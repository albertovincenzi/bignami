package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// parsed is the result of reading a headless command line. Flags can stand anywhere.
type parsed struct {
	opts    RunOpts
	json    bool
	words   []string          // the first one is the command
	kv      map[string]string // name=value arguments
	showAll bool
}

func ParseArgs(args []string) (parsed, error) {
	p := parsed{kv: map[string]string{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			if k, v, found := strings.Cut(a, "="); found && len(p.words) > 0 {
				p.kv[k] = v
			} else {
				p.words = append(p.words, a)
			}
			continue
		}
		name, val, hasVal := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		next := func() (string, error) {
			if hasVal {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("--%s needs a value", name)
			}
			i++
			return args[i], nil
		}
		var err error
		switch name {
		case "profile":
			p.opts.profile, err = next()
		case "region":
			p.opts.region, err = next()
		case "confirm":
			p.opts.confirm, err = next()
		case "code":
			p.kv["code"], err = next()
		case "allow-write":
			p.opts.allowWrite = true
		case "json":
			p.json = true
		case "all":
			p.showAll = true
		case "timeout":
			var s string
			if s, err = next(); err == nil {
				p.opts.timeout, err = time.ParseDuration(s)
			}
		default:
			err = fmt.Errorf("unknown flag --%s", name)
		}
		if err != nil {
			return p, err
		}
	}
	return p, nil
}

// CLI handles the headless subcommands. It returns false when the TUI must start.
func CLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "status", "login", "run", "list", "mcp":
	default:
		return false
	}
	p, err := ParseArgs(args[1:])
	if err != nil {
		fail(err)
	}
	switch args[0] {
	case "status":
		info := Status(p.opts.profile, p.opts.region)
		if p.json {
			b, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Println(humanStatus(info))
		}
		if !info.MFAValid {
			os.Exit(2) // a script can test "is the session usable"
		}
	case "login":
		code := p.kv["code"]
		if code == "" {
			fmt.Fprint(os.Stderr, "MFA code: ")
			code, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			code = strings.TrimSpace(code)
		}
		if err := loginCmd(p.opts.profile, code); err != nil {
			fail(err)
		}
		fmt.Println(humanStatus(Status(p.opts.profile, p.opts.region)))
	case "list":
		for _, n := range sortedNames() {
			c, _ := FindCmd(n)
			if c.write && !p.showAll {
				continue
			}
			fmt.Printf("%-48s %s\n", describe(c), c.desc)
		}
		if !p.showAll {
			fmt.Println("\n(commands that write are hidden: use --all to list them)")
		}
	case "run":
		if len(p.words) == 0 {
			fail(fmt.Errorf("usage: bignami run <command> [name=value ...]  (see: bignami list)"))
		}
		c, found := FindCmd(p.words[0])
		if !found {
			fail(fmt.Errorf("unknown command %q (see: bignami list --all)", p.words[0]))
		}
		text, err := RunCmd(c, p.kv, p.opts)
		fmt.Print(text)
		if text != "" && !strings.HasSuffix(text, "\n") {
			fmt.Println()
		}
		if err != nil {
			fail(err)
		}
	case "mcp":
		if err := ServeMCP(os.Stdin, os.Stdout, p.opts); err != nil && err != io.EOF {
			fail(err)
		}
	}
	return true
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "bignami:", err)
	os.Exit(1)
}
