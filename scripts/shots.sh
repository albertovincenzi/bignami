#!/bin/sh
# Regenerates the README screenshots in assets/ from the real UI, with made-up data.
set -eu
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
BIGNAMI_SHOTS="$tmp" go test -run TestRenderShots ./... >/dev/null
python3 scripts/ansi2svg.py "$tmp/home.ansi" assets/screenshot-home.svg "bignami"
python3 scripts/ansi2svg.py "$tmp/sqs.ansi" assets/screenshot-sqs.svg "bignami · AWS › SQS"
python3 scripts/ansi2svg.py "$tmp/s3.ansi" assets/screenshot-s3.svg "bignami · AWS › S3"
rm -rf "$tmp"
