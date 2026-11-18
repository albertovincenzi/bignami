package awsprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/albertovincenzi/bignami/internal/shell"
)

const MFASuffix = "-mfa"

const ExpirationKey = "x_bignami_expiration"

const SessionSecs = "43200"

var MfaCodeRe = regexp.MustCompile(`^\d{6}$`)

// Env selects a profile and drops static credentials from the environment,
// because they would override the profile.
func Env(profile, region string) []string {
	var env []string
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "AWS_ACCESS_KEY_ID="),
			strings.HasPrefix(e, "AWS_SECRET_ACCESS_KEY="),
			strings.HasPrefix(e, "AWS_SESSION_TOKEN="),
			strings.HasPrefix(e, "AWS_PROFILE="),
			strings.HasPrefix(e, "AWS_REGION="),
			strings.HasPrefix(e, "AWS_DEFAULT_REGION="):
		default:
			env = append(env, e)
		}
	}
	env = append(env, "AWS_PROFILE="+profile)
	if region != "" {
		env = append(env, "AWS_REGION="+region, "AWS_DEFAULT_REGION="+region)
	}
	return env
}

// DetectRegion reads the region from the environment, then from the profile.
func DetectRegion(profile string) string {
	for _, k := range []string{"AWS_REGION", "AWS_DEFAULT_REGION"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	s, _ := shell.Output(Env(profile, ""), "aws", "configure", "get", "region") // unset region: empty, the profile default applies
	return strings.TrimSpace(s)
}

// Login gets a 12h MFA session for the base profile and saves it as
// "<base>-mfa" in the shared credentials file.
func Login(base, code string) error {
	if !MfaCodeRe.MatchString(code) {
		return fmt.Errorf("the MFA code must have 6 digits")
	}
	env := Env(base, "")
	serial := os.Getenv("BIGNAMI_MFA_SERIAL")
	if serial == "" {
		s, err := shell.Output(env, "aws", "iam", "list-mfa-devices",
			"--query", "MFADevices[0].SerialNumber", "--output", "text")
		if err != nil {
			return err
		}
		serial = strings.TrimSpace(s)
		if serial == "" || serial == "None" {
			return fmt.Errorf("profile %q has no MFA device (set BIGNAMI_MFA_SERIAL)", base)
		}
	}
	s, err := shell.Output(env, "aws", "sts", "get-session-token",
		"--serial-number", serial, "--token-code", code, "--duration-seconds", SessionSecs,
		"--query", "Credentials.[AccessKeyId,SecretAccessKey,SessionToken,Expiration]",
		"--output", "text")
	if err != nil {
		return err
	}
	f := strings.Fields(s)
	if len(f) != 4 {
		return fmt.Errorf("unexpected get-session-token output")
	}
	return WriteProfile(base+MFASuffix, [][2]string{
		{"aws_access_key_id", f[0]},
		{"aws_secret_access_key", f[1]},
		{"aws_session_token", f[2]},
		{ExpirationKey, f[3]},
	})
}

// Remaining returns the time left of a session profile written by awsLogin.
func Remaining(profile string) (time.Duration, bool) {
	t, err := time.Parse(time.RFC3339, ReadProfile(profile)[ExpirationKey])
	if err != nil {
		return 0, false
	}
	return time.Until(t), true
}

func Profiles() ([]string, error) {
	s, err := shell.Output(nil, "aws", "configure", "list-profiles")
	if err != nil {
		return nil, err
	}
	return strings.Fields(s), nil
}

type Section struct {
	name  string
	lines []string // without the header line
}

func CredsPath() string {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "credentials")
}

// SplitSections returns the preamble (name "") followed by the sections.
func SplitSections(text string) []Section {
	secs := []Section{{}}
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			secs = append(secs, Section{name: t[1 : len(t)-1]})
			continue
		}
		last := &secs[len(secs)-1]
		last.lines = append(last.lines, ln)
	}
	return secs
}

func JoinSections(secs []Section) string {
	var lines []string
	for i, s := range secs {
		if i > 0 {
			lines = append(lines, "["+s.name+"]")
		}
		lines = append(lines, s.lines...)
	}
	return strings.Join(lines, "\n")
}

func ReadProfile(name string) map[string]string {
	kv := map[string]string{}
	b, err := os.ReadFile(CredsPath())
	if err != nil {
		return kv
	}
	for _, s := range SplitSections(string(b)) {
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

// WriteProfile replaces (or appends) one section and writes the file atomically with mode 0600.
func WriteProfile(name string, kv [][2]string) error {
	path := CredsPath()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var kept []Section
	for _, s := range SplitSections(string(b)) {
		if s.name != name || s.name == "" {
			kept = append(kept, s)
		}
	}
	lines := make([]string, 0, len(kv)+1)
	for _, p := range kv {
		lines = append(lines, p[0]+" = "+p[1])
	}
	lines = append(lines, "")
	kept = append(kept, Section{name: name, lines: lines})
	text := strings.TrimLeft(JoinSections(kept), "\n")

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
