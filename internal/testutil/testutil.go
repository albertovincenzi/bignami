// Package testutil holds helpers shared by the tests of several packages.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// StubAWS puts a silent `aws` first in PATH and gives the session a temporary credentials file.
func StubAWS(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aws"), []byte("#!/bin/sh\necho \"aws $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin:/usr/sbin:/sbin")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_REGION", "eu-west-1")
}
