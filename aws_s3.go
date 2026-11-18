package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	maxPresignSecs = 604800 // the S3 limit: 7 days
	defaultPresign = 3600
)

var s3Product = product{name: "S3", desc: "buckets, objects, links, audit", icon: "▣", color: cAWS, items: s3Items}

func s3Items() []list.Item {
	a := acmd("▣")
	return []list.Item{
		a("Buckets", "pick a bucket, then an action", pickAWS("buckets",
			[]string{"s3api", "list-buckets", "--query", "Buckets[].Name", "--output", "text"}, "▣", nil, s3Bucket)),
		a("List buckets (table)", "name and creation date", do(
			`aws s3api list-buckets --query 'Buckets[].[Name,CreationDate]' --output table`)),
	}
}

func s3Bucket(m *model, b string) tea.Cmd {
	a := acmd("▸")
	key := promptSpec{label: "Object key"}
	m.push(b, []list.Item{
		a("Browse", "list a prefix", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Prefix (optional)", optional: true}}, func(m *model, v []string) tea.Cmd {
				return m.sh(s3BrowseScript(b, v[0]))
			})
		}),
		a("Security audit", "bucket exposure and protection", do(s3AuditScript(b))),
		a("Size and object count", "from CloudWatch (bucket's region)", do(s3SizeScript(b))),
		a("Presigned link", "temporary download URL", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key, {label: "Expires in seconds (default 3600)", optional: true}}, func(m *model, v []string) tea.Cmd {
				secs, err := parseExpires(v[1])
				if err != nil {
					m.err = err.Error()
					return nil
				}
				return m.sh(s3PresignScript(b, v[0], secs))
			})
		}),
		a("Preview an object", "first 16 KB as text", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd { return m.sh(s3PreviewScript(b, v[0])) })
		}),
		a("Object metadata", "type, size, encryption, class", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd { return m.sh(s3HeadScript(b, v[0])) })
		}),
		a("Object versions", "versions and delete markers of a key", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd { return m.sh(s3VersionsScript(b, v[0])) })
		}),
		a("Restore a deleted object", "removes the latest delete marker", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This changes the bucket", v[0], func(m *model) tea.Cmd { return m.sh(s3UndeleteScript(b, v[0])) })
			})
		}),
		a("Download", "copy an object to a local folder", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key, {label: "Local folder (default ~/Downloads)", optional: true}}, func(m *model, v []string) tea.Cmd {
				dest := expandHome(v[1])
				if dest == "" {
					dest = expandHome("~/Downloads")
				}
				return m.sh(s3DownloadScript(b, v[0], dest))
			})
		}),
		a("Upload", "copy a local file into the bucket", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{{label: "Local file"}, {label: "Object key (default: the file name)", optional: true}}, func(m *model, v []string) tea.Cmd {
				local := expandHome(v[0])
				if st, err := os.Stat(local); err != nil || st.IsDir() {
					m.err = "not a readable file: " + local
					return nil
				}
				k := v[1]
				if k == "" {
					k = filepath.Base(local)
				}
				return m.confirmName("This writes s3://"+b+"/"+k, k, func(m *model) tea.Cmd { return m.sh(s3UploadScript(b, k, local)) })
			})
		}),
	})
	return nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func parseExpires(v string) (int, error) {
	if v == "" {
		return defaultPresign, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > maxPresignSecs {
		return 0, fmt.Errorf("expiry must be a number from 1 to %d seconds", maxPresignSecs)
	}
	return n, nil
}

func s3Uri(bucket, key string) string { return "s3://" + bucket + "/" + key }

func s3BrowseScript(bucket, prefix string) string {
	return "aws s3 ls " + shQuote(s3Uri(bucket, strings.TrimPrefix(prefix, "/"))) + " --human-readable"
}

func s3PresignScript(bucket, key string, secs int) string {
	return fmt.Sprintf("aws s3 presign %s --expires-in %d", shQuote(s3Uri(bucket, key)), secs)
}

func s3HeadScript(bucket, key string) string {
	return "aws s3api head-object --bucket " + shQuote(bucket) + " --key " + shQuote(key)
}

// s3PreviewScript reads a ranged slice, so a huge object is never downloaded, and masks control bytes.
func s3PreviewScript(bucket, key string) string {
	return `T=$(mktemp) && trap 'rm -f "$T"' EXIT && aws s3api get-object --bucket ` + shQuote(bucket) +
		` --key ` + shQuote(key) + ` --range bytes=0-16383 "$T" >/dev/null && LC_ALL=C tr -c '[:print:]\n\t' '.' < "$T" | head -n 80`
}

func s3VersionsScript(bucket, key string) string {
	return "aws s3api list-object-versions --bucket " + shQuote(bucket) + " --prefix " + shQuote(key) +
		` --query '{Versions:Versions[].[VersionId,IsLatest,LastModified,Size],DeleteMarkers:DeleteMarkers[].[VersionId,IsLatest,LastModified]}'`
}

func s3UndeleteScript(bucket, key string) string {
	return `B=` + shQuote(bucket) + `; K=` + shQuote(key) + `
M=$(aws s3api list-object-versions --bucket "$B" --prefix "$K" --query 'DeleteMarkers[?IsLatest].[Key,VersionId]' --output text | awk -F'\t' -v k="$K" '$1==k{print $2}')
[ -n "$M" ] || { echo "no delete marker on top of $K"; exit 1; }
aws s3api delete-object --bucket "$B" --key "$K" --version-id "$M" && echo "restored: $K"`
}

func s3DownloadScript(bucket, key, destDir string) string {
	return "mkdir -p " + shQuote(destDir) + " && aws s3 cp " + shQuote(s3Uri(bucket, key)) + " " + shQuote(destDir+"/")
}

func s3UploadScript(bucket, key, local string) string {
	return "aws s3 cp " + shQuote(local) + " " + shQuote(s3Uri(bucket, key))
}

// s3AuditScript prints the settings that decide whether a bucket is exposed or protected.
func s3AuditScript(bucket string) string {
	return `B=` + shQuote(bucket) + `; pg() { printf '\n== %s ==\n' "$1"; shift; "$@" 2>&1 | head -40; }
pg head-bucket aws s3api head-bucket --bucket "$B"
pg versioning aws s3api get-bucket-versioning --bucket "$B"
pg public-access-block aws s3api get-public-access-block --bucket "$B"
pg policy aws s3api get-bucket-policy --bucket "$B" --query Policy --output text
pg logging aws s3api get-bucket-logging --bucket "$B"
pg object-lock aws s3api get-object-lock-configuration --bucket "$B"`
}

func s3SizeScript(bucket string) string {
	return `B=` + shQuote(bucket) + `
S=$(date -u -v-3d +%FT%TZ 2>/dev/null || date -u -d '3 days ago' +%FT%TZ); E=$(date -u +%FT%TZ)
metric() { aws cloudwatch get-metric-statistics --namespace AWS/S3 --metric-name "$1" --dimensions Name=BucketName,Value="$B" Name=StorageType,Value="$2" --start-time "$S" --end-time "$E" --period 86400 --statistics Average --query 'sort_by(Datapoints,&Timestamp)[-1].[Timestamp,Average]' --output text; }
echo "size in bytes (standard storage), latest daily point:"; metric BucketSizeBytes StandardStorage
echo "number of objects, latest daily point:"; metric NumberOfObjects AllStorageTypes`
}
