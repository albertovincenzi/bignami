package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/albertovincenzi/bignami/internal/agent"
	"github.com/albertovincenzi/bignami/internal/tui"
	"github.com/albertovincenzi/bignami/internal/version"
)

const usage = `bignami: a TUI cheat sheet for AWS and Kubernetes commands.
Run it without arguments for the TUI.

For scripts and agents:
  bignami status [--json]              profile, region and MFA session (exit 2 if the session is missing)
  bignami login --code 123456          start a 12h MFA session
  bignami list [--all]                 headless commands (--all adds the ones that write)
  bignami run <command> [name=value]   run one command, for example: bignami run s3.audit bucket=my-bucket
  bignami mcp [--allow-write]          MCP server on stdio, for Claude, Codex and others
Flags: --profile --region --confirm <value> --allow-write --timeout 30s  -v, --version  -h, --help`

func main() {
	if bi, ok := debug.ReadBuildInfo(); ok && version.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version.Version = bi.Main.Version
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Println("bignami", version.Version)
			return
		case "-h", "--help", "help":
			fmt.Println(usage)
			return
		}
	}
	if agent.CLI(os.Args[1:]) {
		return
	}
	if err := tui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
