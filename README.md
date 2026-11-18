<p align="center">
  <img alt="bignami" src="assets/logo-dark.svg#gh-dark-mode-only" width="460">
  <img alt="bignami" src="assets/logo-light.svg#gh-light-mode-only" width="460">
</p>

<p align="center">
  <b>You pick a product. You pick a command. The output appears next to the list.<br>No flags to remember, no docs to open.</b>
</p>

<p align="center">
  <a href="https://github.com/albertovincenzi/bignami/releases"><img alt="release" src="https://img.shields.io/github/v/release/albertovincenzi/bignami?color=6741EA&labelColor=0F172A"></a>
  <a href="https://github.com/albertovincenzi/bignami/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/albertovincenzi/bignami/ci.yml?label=CI&color=6741EA&labelColor=0F172A"></a>
  <img alt="platforms" src="https://img.shields.io/badge/macOS%20%C2%B7%20Linux-6741EA?labelColor=0F172A">
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-6741EA?labelColor=0F172A"></a>
</p>

<p align="center">
  <img alt="bignami home screen" src="assets/screenshot-home.svg" width="760">
</p>

```sh
brew install albertovincenzi/tap/bignami
```

## Why

You know what you want: "who is blocking this email address?", "why is this queue full?", "is this bucket public?". You do not remember the exact `aws` command. bignami keeps those commands for you, asks only for the values it needs, and shows the result at once.

- **Pick, do not type.** Buckets, topics, queues and roles come from a list. No ARNs to paste.
- **Safe by default.** Anything that deletes, sends or moves data asks you to type the name first. Prod contexts turn red.
- **It remembers.** The last value you typed for each prompt, and your AWS profile and region, are there next time.
- **Region and profile in one key.** Press `r` on any AWS screen.
- **Never in your way.** Your kubeconfig and gcloud config are not touched. MFA codes are not logged.

<p align="center">
  <img alt="SQS dead-letter queue, peeking at messages" src="assets/screenshot-sqs.svg" width="760">
</p>

## What it does

Six domains. Each one has products, and each product has its commands.

| Domain | Product | Commands |
|---|---|---|
| **AWS** | Session | MFA login (12h), who am I, switch profile, switch region, shell with the profile, custom `aws` command |
| | S3 | Pick a bucket: browse, security audit, size, presigned link, preview, metadata, versions, restore a deleted object, download, upload |
| | SES | Reputation (bounce and complaint rate), send statistics, identities (details, DKIM records, policies), suppression list, configuration sets, templates, account, test email |
| | SNS | Pick a topic: subscriptions, filter policies, attributes, test message |
| | SQS | Queues with message counts (a non-empty DLQ turns red), peek, send, redrive a DLQ, redrive status, purge |
| | IAM | My access, "can I do this?" simulation, access key age, MFA devices, roles |
| **Kubernetes** | Cluster | Switch context and namespace, namespaces, nodes, node usage |
| | Workloads | Pods (an empty namespace offers the ones that have pods), unhealthy pods, most restarts, deployments, usage |
| | Debug | Warnings of the whole cluster, events, port-forward, custom `kubectl` command |
| **GitHub** | Pull requests, Actions, Repo | My PRs, PRs to review, checks; runs, failed logs, re-run; releases, issues (uses `gh` in the current folder) |
| **Helm** | Releases, Charts | Releases, status, history, values, manifest, rollback; repos and search |
| **Google Cloud** | Account, GKE, Secrets, Logs | Who am I, switch project, clusters, credentials, secrets, log queries |
| **Docker** | Containers, Images and disk, Compose | Logs, shell, stats, restart, stop; disk usage, clean up; services of a Compose file |

<p align="center">
  <img alt="SES reputation: bounce and complaint rates" src="assets/screenshot-ses.svg" width="760">
</p>

<p align="center">
  <img alt="S3 bucket security audit" src="assets/screenshot-s3.svg" width="760">
</p>

## Install

You need the CLIs of the domains you use in your `PATH`: [AWS CLI](https://aws.amazon.com/cli/), `kubectl`, `gh`, `helm`, `gcloud`, `docker`. macOS and Linux are supported.

With Homebrew:

```sh
brew install albertovincenzi/tap/bignami
```

If Homebrew says the tap is untrusted, run `brew trust albertovincenzi/tap` and install again.

With Go:

```sh
go install github.com/albertovincenzi/bignami@latest
```

Or download a binary from the [releases page](https://github.com/albertovincenzi/bignami/releases).

## Verify a download

Each release has a keyless [Sigstore](https://www.sigstore.dev) signature of `checksums.txt`, made by the release workflow. It proves the files come from this repository. Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/), then:

```sh
v=0.2.1   # the version you downloaded, with checksums.txt and checksums.txt.sigstore.json from the same release
cosign verify-blob \
  --certificate-identity "https://github.com/albertovincenzi/bignami/.github/workflows/release.yml@refs/tags/v$v" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle checksums.txt.sigstore.json checksums.txt
shasum -a 256 --check --ignore-missing checksums.txt
```

The binaries are not signed with an Apple Developer ID. Homebrew installs do not need that. A binary downloaded with a browser may need `xattr -d com.apple.quarantine bignami`.

## Use

```sh
bignami
```

| Key | Action |
|---|---|
| `←` `→` `↑` `↓` or `1`-`6` | Choose a domain |
| `enter` | Open a domain or product, or run a command |
| `r` | Change the AWS region (AWS screens) |
| `m` | MFA login (AWS screens). bignami suggests it when a command is denied for lack of MFA |
| `/` | Filter the list |
| `tab` | Move the focus to the output pane (scroll with `↑↓`, `PgUp`, `PgDn`, `g`, `G`) |
| `x` | Stop the running command |
| `esc` | Go back |
| `q` | Quit (it stops running commands first) |

The output pane sits on the right when the terminal is at least 110 columns wide. On a narrow terminal it replaces the list while a command runs.

Commands that need a real terminal, such as a shell in a pod, take over the screen. When you exit them, you come back to bignami.

## AWS and MFA

The MFA session survives a restart: bignami remembers your profile and region and uses the `<name>-mfa` profile while it is valid. The MFA login uses the profile shown in the AWS card. If you do not use `AWS_PROFILE`, it uses `default`.

1. It reads your MFA device serial with `iam list-mfa-devices`. You can set it yourself with `BIGNAMI_MFA_SERIAL`.
2. It asks for the 6-digit code and calls `sts get-session-token`.
3. It writes the session to the profile `<name>-mfa` in `~/.aws/credentials`. Only that section changes. The file keeps the mode `0600`.

A program cannot change the environment of the shell that started it. So use the new profile with `export AWS_PROFILE=<name>-mfa`, or pick "Shell with this profile".

## For scripts and agents

Claude Code, Codex and scripts can use the same commands without the TUI. An MFA code needs a human, so the flow is: you type the code once, and the agent uses the 12h session.

```sh
bignami status --json                 # profile, region, MFA time left; exit code 2 if the session is missing
bignami login --code 123456           # start the 12h session (you type the code)
bignami list                          # the commands (add --all to see the ones that write)
bignami run s3.audit bucket=my-bucket # run one command
```

A command that changes data needs `--allow-write` and `--confirm <name>`, for example `bignami run sqs.purge queue_url=<url> --allow-write --confirm <queue name>`.

`status` tells an agent what to do next: when the session is expired, `next_step` says to ask you for the code.

**MCP server.** `bignami mcp` speaks MCP on stdio and exposes every command as a tool.

```sh
claude mcp add bignami -- bignami mcp
```

For Codex, add this to `~/.codex/config.toml`:

```toml
[mcp_servers.bignami]
command = "bignami"
args = ["mcp"]
```

It is **read-only** by default: audits, SES reputation and statistics, queue depth, message peek, IAM simulation, pod logs and unhealthy pods, CI runs, Helm releases, Docker logs. Secret values are never exposed. Commands that change data (purge, redrive, send, publish, upload, undelete, re-run a CI run, Helm rollback, restart a container) appear only with `bignami mcp --allow-write`, and each one needs a `confirm` value, the same name you type in the TUI. Output is cut at 256 KB and every call stops after 60 seconds.

Keep in mind that an agent with your MFA session can read what you can read.

## Safety

- bignami keeps its own Kubernetes context, namespace and Google Cloud project, and passes them on every call. Your kubeconfig and gcloud config are not changed, except when you ask for "Get credentials" (it asks you to confirm).
- If the context name contains `prod`, the card turns red. Opening a shell or running a free `kubectl` command asks you to type `prod` first.
- Commands that delete, purge, send, upload or move data ask you to type the name of the target first.
- The MFA code and the session token are never written to logs.

## Development

```sh
go test ./...
go run .
```

The code is small: `main.go` (state and keys), `domains.go` (domains and products), `aws_*.go`, `k8s*.go` and `tools_*.go` (the commands of each domain), `prompts.go` and `recents.go` (input), `pane.go` (the output pane), `ui.go` (the look). `headless*.go`, `cli.go` and `mcp.go` serve scripts and agents with the same scripts as the TUI.

The screenshots above come from the real UI with made-up data: run `scripts/shots.sh` to regenerate them.

## License

MIT
