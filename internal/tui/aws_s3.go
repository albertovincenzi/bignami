package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertovincenzi/bignami/internal/scripts"
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
				return m.sh(scripts.S3BrowseScript(b, v[0]))
			})
		}),
		a("Security audit", "bucket exposure and protection", do(scripts.S3AuditScript(b))),
		a("Size and object count", "from CloudWatch (bucket's region)", do(scripts.S3SizeScript(b))),
		a("Presigned link", "temporary download URL", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key, {label: "Expires in seconds (default 3600)", optional: true}}, func(m *model, v []string) tea.Cmd {
				secs, err := scripts.ParseExpires(v[1])
				if err != nil {
					m.err = err.Error()
					return nil
				}
				return m.sh(scripts.S3PresignScript(b, v[0], secs))
			})
		}),
		a("Preview an object", "first 16 KB as text", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd { return m.sh(scripts.S3PreviewScript(b, v[0])) })
		}),
		a("Object metadata", "type, size, encryption, class", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd { return m.sh(scripts.S3HeadScript(b, v[0])) })
		}),
		a("Object versions", "versions and delete markers of a key", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd { return m.sh(scripts.S3VersionsScript(b, v[0])) })
		}),
		a("Restore a deleted object", "removes the latest delete marker", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key}, func(m *model, v []string) tea.Cmd {
				return m.confirmName("This changes the bucket", v[0], func(m *model) tea.Cmd { return m.sh(scripts.S3UndeleteScript(b, v[0])) })
			})
		}),
		a("Download", "copy an object to a local folder", func(m *model) tea.Cmd {
			return m.ask([]promptSpec{key, {label: "Local folder (default ~/Downloads)", optional: true}}, func(m *model, v []string) tea.Cmd {
				dest := expandHome(v[1])
				if dest == "" {
					dest = expandHome("~/Downloads")
				}
				return m.sh(scripts.S3DownloadScript(b, v[0], dest))
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
				return m.confirmName("This writes s3://"+b+"/"+k, k, func(m *model) tea.Cmd { return m.sh(scripts.S3UploadScript(b, k, local)) })
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
