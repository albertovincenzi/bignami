package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestRenderShots writes the screens used in the README as ANSI text, with made-up data.
// Run: BIGNAMI_SHOTS=/tmp/shots go test -run TestRenderShots ; then scripts/shots.sh converts them to SVG.
func TestRenderShots(t *testing.T) {
	dir := os.Getenv("BIGNAMI_SHOTS")
	if dir == "" {
		t.Skip("set BIGNAMI_SHOTS=<dir> to write the README screenshots")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	exp := time.Now().Add(9*time.Hour + 24*time.Minute).UTC().Format(time.RFC3339)
	if err := writeProfile("dev-mfa", [][2]string{{expirationKey, exp}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	newShot := func(w, h int) *model {
		m := newModel()
		m.awsProfile, m.awsRegion = "dev-mfa", "eu-west-1"
		m.kubeCtx = "gke_acme-stage_europe-west1_k8s-01-stage"
		m.w, m.h = w, h
		return m
	}
	save := func(name string, m *model) {
		v := strings.TrimRight(m.View(), "\n")
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	show := func(m *model, title string, lines ...string) {
		m.run = &run{id: 1, title: title, lines: lines, done: true, elapsed: 800 * time.Millisecond}
		m.showPane = true
		m.layout()
	}
	open := func(m *model, pathTitles ...string) {
		m.openDomain()
		for _, p := range pathTitles {
			m.push(p, []list.Item{item{title: p}})
		}
	}

	home := newShot(118, 36)
	save("home", home)

	sqs := newShot(130, 36)
	open(sqs, "SQS", "queues")
	sqsQueue(sqs, "https://sqs.eu-west-1.amazonaws.com/123456789012/orders-dlq")
	sqs.cur().Select(1)
	show(sqs, "aws sqs receive-message --queue-url 'https://sqs.eu-west-1.amazonaws.com/123456789012/orders-dlq' --max-number-of-messages 10 ...",
		"{", `    "Messages": [`, "        {",
		`            "MessageId": "7f1c2a9e-5d3b-4c8e-9a10-2b6e0d4f8a31",`,
		`            "Body": "{\"orderId\":\"ord_10492\",\"event\":\"payment.failed\"}",`,
		`            "Attributes": {`,
		`                "ApproximateReceiveCount": "6",`,
		`                "SentTimestamp": "1731912000000"`,
		"            }", "        }", "    ]", "}")
	save("sqs", sqs)

	s3 := newShot(130, 36)
	open(s3, "S3", "buckets")
	s3Bucket(s3, "acme-uploads")
	s3.cur().Select(1)
	show(s3, "bucket audit: acme-uploads",
		"", "== head-bucket ==", "", "== versioning ==", `{ "Status": "Enabled" }`, "",
		"== public-access-block ==", `{ "BlockPublicAcls": true, "IgnorePublicAcls": true,`,
		`  "BlockPublicPolicy": true, "RestrictPublicBuckets": true }`, "",
		"== policy ==", "An error occurred (NoSuchBucketPolicy): The bucket policy does not exist", "",
		"== logging ==", `{ "LoggingEnabled": { "TargetBucket": "acme-logs", "TargetPrefix": "s3/" } }`)
	save("s3", s3)
}
