package main

import (
	"errors"
	"testing"
	"time"
)

const mfaDenied = "An error occurred (AccessDeniedException) when calling the ListEmailIdentities operation: User: arn:aws:iam::1:user/x is not authorized to perform: ses:ListEmailIdentities with an explicit deny in an identity-based policy: arn:aws:iam::1:policy/EnforceMFAForAdmins"

func TestMFAHintOnlyWhenTheSessionIsMissing(t *testing.T) {
	stubAWS(t)
	m := newModel()
	m.awsProfile = "dev"
	if m.mfaHint(mfaDenied) == "" {
		t.Error("an MFA denial without a session must suggest the login")
	}
	if m.mfaHint("An error occurred (AccessDeniedException): not authorized to perform: s3:GetObject") != "" {
		t.Error("a denial that does not name MFA must not suggest the login")
	}
	exp := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if err := writeProfile("dev-mfa", [][2]string{{expirationKey, exp}}); err != nil {
		t.Fatal(err)
	}
	m.awsProfile = "dev-mfa"
	if m.mfaHint(mfaDenied) != "" {
		t.Error("with a valid MFA session the denial is a real missing permission: no hint")
	}
	m.awsProfile, m.sel = "dev", 1
	if m.mfaHint(mfaDenied) != "" {
		t.Error("the hint is for AWS only")
	}
}

func TestErrorShowsTheMFAHint(t *testing.T) {
	stubAWS(t)
	m := newModel()
	m.awsProfile = "dev"
	m.Update(errMsg{errors.New(mfaDenied)})
	if m.err != "this needs an MFA session: press m to log in" {
		t.Errorf("err = %q", m.err)
	}
}

func TestMFASessionAndProfileSurviveARestart(t *testing.T) {
	stubAWS(t)
	t.Setenv("AWS_REGION", "")
	first := newModel()
	first.recents.set(recentProfile, "dev")
	first.recents.set(recentRegion, "eu-central-1")
	exp := time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339)
	if err := writeProfile("dev-mfa", [][2]string{{expirationKey, exp}}); err != nil {
		t.Fatal(err)
	}
	second := newModel() // a new process: nothing in memory
	second.detect()
	if second.awsProfile != "dev-mfa" || second.awsRegion != "eu-central-1" {
		t.Errorf("after a restart: profile %q, region %q", second.awsProfile, second.awsRegion)
	}
}
