# bignami

A terminal UI for devs who are lazy, forgetful, or both. You pick a domain, you pick a command, and the output shows up next to the list. You do not have to remember the flags.

```
 bignami  a cheat sheet for forgetful devs

 ╭─ ◆ AWS ─────────────────╮ ╭─ ⬢ KUBERNETES ──────────╮
 │ default-mfa             │ │ my-project · cluster-01 │
 │ ██████████░░ 11h42m     │ │ namespace default       │
 ╰─────────────────────────╯ ╰─────────────────────────╯

 AWS                          ╭─ $ aws sts get-caller-identity ─╮
 ▌ ◆ MFA login                │ ✓ done in 0.6s                  │
 ◆ Who am I                   │ ──────────────────────────────  │
 ◆ Switch profile             │ { "UserId": "AIDA...", ... }    │
 ◆ S3: audit a bucket         ╰─────────────────────────────────╯
```

## What it does

**AWS**

| Command | What it runs |
|---|---|
| MFA login | Gets a 12h session token and saves it as the profile `<name>-mfa` |
| Who am I | `sts get-caller-identity` |
| Switch profile | Picks a profile from `~/.aws/config` |
| Shell with this profile | Subshell with `AWS_PROFILE` set |
| My MFA devices and keys | `iam list-mfa-devices`, `iam list-access-keys` |
| S3: list / browse / audit | Buckets, a prefix, and the versioning, public access, policy, logging and object lock settings of one bucket |
| Logs: tail a log group | `logs tail --follow` |
| Decode an access error | `sts decode-authorization-message` |
| Custom aws command | Anything else |

**Kubernetes**

Switch context and namespace, list pods (logs, shell, describe), deployments, recent events, `top pods`, port-forward, and a custom `kubectl` command.

## Install

You need the [AWS CLI](https://aws.amazon.com/cli/) and `kubectl` in your `PATH`. macOS and Linux are supported.

With Homebrew:

```sh
brew tap albertovincenzi/bignami https://github.com/albertovincenzi/bignami
brew trust albertovincenzi/bignami
brew install bignami
```

Recent Homebrew versions refuse to load a formula from a third-party tap until you trust the tap. The `brew trust` line does that. Read the [formula](Formula/bignami.rb) first if you want to check it.

With Go:

```sh
go install github.com/albertovincenzi/bignami@latest
```

Or download a binary from the [releases page](https://github.com/albertovincenzi/bignami/releases).

## Use

```sh
bignami
```

| Key | Action |
|---|---|
| `←` `→` or `1` `2` | Choose a domain |
| `enter` | Open a domain, or run a command |
| `/` | Filter the list |
| `tab` | Move the focus to the output pane (scroll with `↑↓`, `PgUp`, `PgDn`, `g`, `G`) |
| `x` | Stop the running command |
| `esc` | Go back |
| `q` | Quit (it stops running commands first) |

The output pane sits on the right when the terminal is at least 110 columns wide. On a narrow terminal it replaces the list while a command runs.

Commands that need a real terminal, such as a shell in a pod, take over the screen. When you exit them, you come back to bignami.

## AWS and MFA

The MFA login uses the profile shown in the AWS card. If you do not use `AWS_PROFILE`, it uses `default`.

1. It reads your MFA device serial with `iam list-mfa-devices`. You can set it yourself with `BIGNAMI_MFA_SERIAL`.
2. It asks for the 6-digit code and calls `sts get-session-token`.
3. It writes the session to the profile `<name>-mfa` in `~/.aws/credentials`. Only that section changes. The file keeps the mode `0600`.

A program cannot change the environment of the shell that started it. So use the new profile with `export AWS_PROFILE=<name>-mfa`, or pick "Shell with this profile".

## Safety

- bignami never changes your kubeconfig. It keeps its own context and namespace, and it passes `--context` on every call.
- If the context name contains `prod`, the card turns red. Opening a shell or running a free `kubectl` command asks you to type `prod` first.
- The MFA code and the session token are never written to logs.

## Development

```sh
go test ./...
go run .
```

The code is small: `main.go` (state and keys), `domains.go` (the commands), `pane.go` (the output pane), `ui.go` (the look), `aws.go` and `k8s.go` (the helpers).

## License

MIT
