package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	mfaSuffix     = "-mfa"
	expirationKey = "x_bignami_expiration"
	sessionSecs   = "43200"
)

var mfaCodeRe = regexp.MustCompile(`^\d{6}$`)

// awsEnv selects a profile and drops static credentials from the environment,
// because they would override the profile.
func awsEnv(profile string) []string {
	var env []string
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "AWS_ACCESS_KEY_ID="),
			strings.HasPrefix(e, "AWS_SECRET_ACCESS_KEY="),
			strings.HasPrefix(e, "AWS_SESSION_TOKEN="),
			strings.HasPrefix(e, "AWS_PROFILE="):
		default:
			env = append(env, e)
		}
	}
	return append(env, "AWS_PROFILE="+profile)
}

// awsLogin gets a 12h MFA session for the base profile and saves it as
// "<base>-mfa" in the shared credentials file.
func awsLogin(base, code string) error {
	if !mfaCodeRe.MatchString(code) {
		return fmt.Errorf("the MFA code must have 6 digits")
	}
	env := awsEnv(base)
	serial := os.Getenv("BIGNAMI_MFA_SERIAL")
	if serial == "" {
		s, err := out(env, "aws", "iam", "list-mfa-devices",
			"--query", "MFADevices[0].SerialNumber", "--output", "text")
		if err != nil {
			return err
		}
		serial = strings.TrimSpace(s)
		if serial == "" || serial == "None" {
			return fmt.Errorf("profile %q has no MFA device (set BIGNAMI_MFA_SERIAL)", base)
		}
	}
	s, err := out(env, "aws", "sts", "get-session-token",
		"--serial-number", serial, "--token-code", code, "--duration-seconds", sessionSecs,
		"--query", "Credentials.[AccessKeyId,SecretAccessKey,SessionToken,Expiration]",
		"--output", "text")
	if err != nil {
		return err
	}
	f := strings.Fields(s)
	if len(f) != 4 {
		return fmt.Errorf("unexpected get-session-token output")
	}
	return writeProfile(base+mfaSuffix, [][2]string{
		{"aws_access_key_id", f[0]},
		{"aws_secret_access_key", f[1]},
		{"aws_session_token", f[2]},
		{expirationKey, f[3]},
	})
}

// awsRemaining returns the time left of a session profile written by awsLogin.
func awsRemaining(profile string) (time.Duration, bool) {
	t, err := time.Parse(time.RFC3339, readProfile(profile)[expirationKey])
	if err != nil {
		return 0, false
	}
	return time.Until(t), true
}

func awsProfiles() ([]string, error) {
	s, err := out(nil, "aws", "configure", "list-profiles")
	if err != nil {
		return nil, err
	}
	return strings.Fields(s), nil
}

// --- credentials file: edit one section, keep the rest of the file as is ---

type section struct {
	name  string
	lines []string // without the header line
}

func credsPath() string {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "credentials")
}

// splitSections returns the preamble (name "") followed by the sections.
func splitSections(text string) []section {
	secs := []section{{}}
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			secs = append(secs, section{name: t[1 : len(t)-1]})
			continue
		}
		last := &secs[len(secs)-1]
		last.lines = append(last.lines, ln)
	}
	return secs
}

func joinSections(secs []section) string {
	var lines []string
	for i, s := range secs {
		if i > 0 {
			lines = append(lines, "["+s.name+"]")
		}
		lines = append(lines, s.lines...)
	}
	return strings.Join(lines, "\n")
}

func readProfile(name string) map[string]string {
	kv := map[string]string{}
	b, err := os.ReadFile(credsPath())
	if err != nil {
		return kv
	}
	for _, s := range splitSections(string(b)) {
		if s.name != name {
			continue
		}
		for _, ln := range s.lines {
			if k, v, ok := strings.Cut(ln, "="); ok {
				kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	return kv
}

// writeProfile replaces (or appends) one section and writes the file atomically with mode 0600.
func writeProfile(name string, kv [][2]string) error {
	path := credsPath()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var kept []section
	for _, s := range splitSections(string(b)) {
		if s.name != name || s.name == "" {
			kept = append(kept, s)
		}
	}
	lines := make([]string, 0, len(kv)+1)
	for _, p := range kv {
		lines = append(lines, p[0]+" = "+p[1])
	}
	lines = append(lines, "")
	kept = append(kept, section{name: name, lines: lines})
	text := strings.TrimLeft(joinSections(kept), "\n")

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
