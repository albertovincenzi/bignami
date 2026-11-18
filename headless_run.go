package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	maxOutput      = 256 << 10 // an agent does not need more, and a huge answer fills its context
	defaultTimeout = 60 * time.Second
)

// runOpts are the choices that every headless call shares.
type runOpts struct {
	profile, region string
	allowWrite      bool
	confirm         string
	timeout         time.Duration
}

// resolveProfile prefers the MFA session of a profile when it is still valid, because accounts that enforce MFA deny everything without it.
func resolveProfile(p string) string {
	if p == "" {
		p = envOr("AWS_PROFILE", "default")
	}
	if !strings.HasSuffix(p, mfaSuffix) {
		if left, ok := awsRemaining(p + mfaSuffix); ok && left > 0 {
			return p + mfaSuffix
		}
	}
	return p
}

// capped keeps the first maxOutput bytes and counts the rest.
type capped struct {
	buf     bytes.Buffer
	dropped int
}

func (c *capped) Write(p []byte) (int, error) {
	room := maxOutput - c.buf.Len()
	if room > len(p) {
		room = len(p)
	}
	if room > 0 {
		c.buf.Write(p[:room])
	}
	c.dropped += len(p) - max(room, 0)
	return len(p), nil
}

func (c *capped) String() string {
	if c.dropped > 0 {
		return fmt.Sprintf("%s\n[output cut: %d more bytes]", c.buf.String(), c.dropped)
	}
	return c.buf.String()
}

// runCmd checks the arguments and the write gate, then runs the script and returns its output.
func runCmd(c cmdSpec, args map[string]string, o runOpts) (string, error) {
	for _, a := range c.args {
		if !a.optional && args[a.name] == "" {
			return "", fmt.Errorf("%s needs %s=<%s>", c.name, a.name, a.desc)
		}
	}
	for k := range args {
		if !hasArg(c, k) {
			return "", fmt.Errorf("%s has no argument %q (arguments: %s)", c.name, k, describe(c))
		}
	}
	if c.write {
		if !o.allowWrite {
			return "", fmt.Errorf("%s changes data: allow it with --allow-write and pass the confirm value", c.name)
		}
		if want := c.confirm(args); o.confirm != want {
			return "", fmt.Errorf("%s needs confirm=%q (%s)", c.name, want, c.needsConfirm())
		}
	}
	profile := resolveProfile(o.profile)
	region := o.region
	if region == "" {
		region = detectRegion(profile)
	}
	call := map[string]string{"_mfa": fmt.Sprint(strings.HasSuffix(profile, mfaSuffix))}
	for k, v := range args {
		call[k] = v
	}
	script, err := c.build(call)
	if err != nil {
		return "", err
	}
	return execScript(awsEnv(profile, region), script, o.timeout)
}

func hasArg(c cmdSpec, name string) bool {
	for _, a := range c.args {
		if a.name == name {
			return true
		}
	}
	return false
}

// execScript runs a shell script with a time limit. The whole process group stops at the limit.
func execScript(env []string, script string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Env = append(env, "PAGER=cat", "AWS_PAGER=")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	var buf capped
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if ctx.Err() != nil {
		return buf.String(), fmt.Errorf("stopped after %s", timeout)
	}
	return buf.String(), err
}

// sessionInfo is what "bignami status" reports.
type sessionInfo struct {
	Profile        string `json:"profile"`         // the profile that commands use
	SessionProfile string `json:"session_profile"` // the MFA profile, if one exists
	Region         string `json:"region"`
	MFAValid       bool   `json:"mfa_session_valid"`
	ExpiresAt      string `json:"expires_at,omitempty"`
	RemainingSecs  int    `json:"remaining_seconds"`
	KubeContext    string `json:"kube_context"`
	NextStep       string `json:"next_step,omitempty"`
}

func status(profile, region string) sessionInfo {
	base := strings.TrimSuffix(envOr("AWS_PROFILE", "default"), mfaSuffix)
	if profile != "" {
		base = strings.TrimSuffix(profile, mfaSuffix)
	}
	used := resolveProfile(profile)
	if region == "" {
		region = detectRegion(used)
	}
	info := sessionInfo{Profile: used, SessionProfile: base + mfaSuffix, Region: region}
	if left, ok := awsRemaining(base + mfaSuffix); ok && left > 0 {
		info.MFAValid = true
		info.RemainingSecs = int(left.Seconds())
		info.ExpiresAt = time.Now().Add(left).UTC().Format(time.RFC3339)
	} else {
		info.NextStep = "the MFA session is missing or expired: ask the user for the 6-digit code, then run: bignami login --profile " + base + " --code <code>"
	}
	if ctx, err := out(nil, "kubectl", "config", "current-context"); err == nil {
		info.KubeContext = strings.TrimSpace(ctx)
	}
	return info
}

func humanStatus(i sessionInfo) string {
	if !i.MFAValid {
		return fmt.Sprintf("profile %s, region %s\nMFA session: none or expired\nnext: %s", i.Profile, i.Region, i.NextStep)
	}
	return fmt.Sprintf("profile %s, region %s\nMFA session: %dh%02dm left\nkube context: %s", i.Profile, i.Region, i.RemainingSecs/3600, i.RemainingSecs%3600/60, i.KubeContext)
}

func loginCmd(profile, code string) error {
	base := strings.TrimSuffix(profile, mfaSuffix)
	if base == "" {
		base = strings.TrimSuffix(envOr("AWS_PROFILE", "default"), mfaSuffix)
	}
	return awsLogin(base, code)
}
