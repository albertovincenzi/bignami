package awsprofile

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

	if err := WriteProfile("dev-mfa", [][2]string{{"aws_access_key_id", "NEW"}, {ExpirationKey, "x"}}); err != nil {
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
	if ReadProfile("dev-mfa")["aws_access_key_id"] != "NEW" {
		t.Error("readProfile did not return the new value")
	}
}

func TestWriteProfileCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "credentials")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", path)
	if err := WriteProfile("p-mfa", [][2]string{{"k", "v"}}); err != nil {
		t.Fatal(err)
	}
	if ReadProfile("p-mfa")["k"] != "v" {
		t.Error("profile not written")
	}
}

func TestAwsLoginRejectsBadCode(t *testing.T) {
	if err := Login("default", "12ab"); err == nil {
		t.Error("expected an error for a non-numeric code")
	}
}

func TestAwsEnvSetsProfileAndRegion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "static")
	t.Setenv("AWS_REGION", "us-east-1")
	env := strings.Join(Env("dev", "eu-west-1"), "\n")
	for _, want := range []string{"AWS_PROFILE=dev", "AWS_REGION=eu-west-1", "AWS_DEFAULT_REGION=eu-west-1"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(env, "AWS_ACCESS_KEY_ID") || strings.Contains(env, "us-east-1") {
		t.Error("static credentials and the old region must be dropped")
	}
	if strings.Contains(strings.Join(Env("dev", ""), "\n"), "AWS_REGION") {
		t.Error("with no region the profile default must apply")
	}
}
