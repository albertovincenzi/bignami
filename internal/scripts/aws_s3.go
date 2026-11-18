package scripts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/albertovincenzi/bignami/internal/shell"
)

const MaxPresignSecs = 604800

const DefaultPresign = 3600

func ParseExpires(v string) (int, error) {
	if v == "" {
		return DefaultPresign, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > MaxPresignSecs {
		return 0, fmt.Errorf("expiry must be a number from 1 to %d seconds", MaxPresignSecs)
	}
	return n, nil
}

func S3Uri(bucket, key string) string { return "s3://" + bucket + "/" + key }

func S3BrowseScript(bucket, prefix string) string {
	return "aws s3 ls " + shell.Quote(S3Uri(bucket, strings.TrimPrefix(prefix, "/"))) + " --human-readable"
}

func S3PresignScript(bucket, key string, secs int) string {
	return fmt.Sprintf("aws s3 presign %s --expires-in %d", shell.Quote(S3Uri(bucket, key)), secs)
}

func S3HeadScript(bucket, key string) string {
	return "aws s3api head-object --bucket " + shell.Quote(bucket) + " --key " + shell.Quote(key)
}

// S3PreviewScript reads a ranged slice, so a huge object is never downloaded, and masks control bytes.
func S3PreviewScript(bucket, key string) string {
	return `T=$(mktemp) && trap 'rm -f "$T"' EXIT && aws s3api get-object --bucket ` + shell.Quote(bucket) +
		` --key ` + shell.Quote(key) + ` --range bytes=0-16383 "$T" >/dev/null && LC_ALL=C tr -c '[:print:]\n\t' '.' < "$T" | head -n 80`
}

func S3VersionsScript(bucket, key string) string {
	return "aws s3api list-object-versions --bucket " + shell.Quote(bucket) + " --prefix " + shell.Quote(key) +
		` --query '{Versions:Versions[].[VersionId,IsLatest,LastModified,Size],DeleteMarkers:DeleteMarkers[].[VersionId,IsLatest,LastModified]}'`
}

func S3UndeleteScript(bucket, key string) string {
	return `B=` + shell.Quote(bucket) + `; K=` + shell.Quote(key) + `
M=$(aws s3api list-object-versions --bucket "$B" --prefix "$K" --query 'DeleteMarkers[?IsLatest].[Key,VersionId]' --output text | awk -F'\t' -v k="$K" '$1==k{print $2}')
[ -n "$M" ] || { echo "no delete marker on top of $K"; exit 1; }
aws s3api delete-object --bucket "$B" --key "$K" --version-id "$M" && echo "restored: $K"`
}

func S3DownloadScript(bucket, key, destDir string) string {
	return "mkdir -p " + shell.Quote(destDir) + " && aws s3 cp " + shell.Quote(S3Uri(bucket, key)) + " " + shell.Quote(destDir+"/")
}

func S3UploadScript(bucket, key, local string) string {
	return "aws s3 cp " + shell.Quote(local) + " " + shell.Quote(S3Uri(bucket, key))
}

// S3AuditScript prints the settings that decide whether a bucket is exposed or protected.
func S3AuditScript(bucket string) string {
	return `B=` + shell.Quote(bucket) + `; pg() { printf '\n== %s ==\n' "$1"; shift; "$@" 2>&1 | head -40; }
pg head-bucket aws s3api head-bucket --bucket "$B"
pg versioning aws s3api get-bucket-versioning --bucket "$B"
pg public-access-block aws s3api get-public-access-block --bucket "$B"
pg policy aws s3api get-bucket-policy --bucket "$B" --query Policy --output text
pg logging aws s3api get-bucket-logging --bucket "$B"
pg object-lock aws s3api get-object-lock-configuration --bucket "$B"`
}

func S3SizeScript(bucket string) string {
	return `B=` + shell.Quote(bucket) + `
S=$(date -u -v-3d +%FT%TZ 2>/dev/null || date -u -d '3 days ago' +%FT%TZ); E=$(date -u +%FT%TZ)
metric() { aws cloudwatch get-metric-statistics --namespace AWS/S3 --metric-name "$1" --dimensions Name=BucketName,Value="$B" Name=StorageType,Value="$2" --start-time "$S" --end-time "$E" --period 86400 --statistics Average --query 'sort_by(Datapoints,&Timestamp)[-1].[Timestamp,Average]' --output text; }
echo "size in bytes (standard storage), latest daily point:"; metric BucketSizeBytes StandardStorage
echo "number of objects, latest daily point:"; metric NumberOfObjects AllStorageTypes`
}
