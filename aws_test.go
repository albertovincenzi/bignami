package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteProfileReplacesOnlyItsSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", path)
	orig := "[default]\naws_access_key_id = AAA\n\n[dev-mfa]\naws_access_key_id = OLD\n\n[other]\nk = v\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeProfile("dev-mfa", [][2]string{{"aws_access_key_id", "NEW"}, {expirationKey, "x"}}); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(path)
	got := string(b)
	for _, want := range []string{"[default]\naws_access_key_id = AAA", "[other]\nk = v", "[dev-mfa]\naws_access_key_id = NEW"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "OLD") || strings.Count(got, "[dev-mfa]") != 1 {
		t.Errorf("old section not replaced:\n%s", got)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", st.Mode().Perm())
	}
	if readProfile("dev-mfa")["aws_access_key_id"] != "NEW" {
		t.Error("readProfile did not return the new value")
	}
}

func TestWriteProfileCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "credentials")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", path)
	if err := writeProfile("p-mfa", [][2]string{{"k", "v"}}); err != nil {
		t.Fatal(err)
	}
	if readProfile("p-mfa")["k"] != "v" {
		t.Error("profile not written")
	}
}

func TestShQuote(t *testing.T) {
	if got := shQuote("it's"); got != `'it'\''s'` {
		t.Errorf("got %s", got)
	}
}

func TestAwsLoginRejectsBadCode(t *testing.T) {
	if err := awsLogin("default", "12ab"); err == nil {
		t.Error("expected an error for a non-numeric code")
	}
}
