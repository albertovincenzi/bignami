package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// kc builds a kubectl command that always carries the context, so the kubeconfig is never changed.
func kc(ctx string, args ...string) string {
	if ctx != "" {
		args = append([]string{"--context", ctx}, args...)
	}
	return shJoin("kubectl", args...)
}

func kcNS(ctx, ns string, args ...string) string {
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	return kc(ctx, args...)
}

// k8sInNS runs a command in one namespace. When the namespace is empty it lists the namespaces that have pods,
// because "No resources found" alone tells nothing.
func k8sInNS(ctx, ns string, args ...string) string {
	hint := kc(ctx, "get", "pods", "-A", "--no-headers", "-o", "custom-columns=NS:.metadata.namespace")
	return `O=$(` + kcNS(ctx, ns, args...) + ` 2>&1); S=$?; echo "$O"
case "$O" in *"No resources found"*)
  echo; echo "Nothing in this namespace. Namespaces that have pods (pods, name):"
  ` + hint + ` | sort | uniq -c | sort -rn
  echo; echo "Pick one with Cluster > Switch namespace.";;
esac
exit $S`
}

// k8sUnhealthy lists the pods that are not ready or not running, in every namespace.
func k8sUnhealthy(ctx string) string {
	return `O=$(` + kc(ctx, "get", "pods", "-A") + `) || exit 1
echo "$O" | awk 'NR==1{print; next} $4=="Completed"{next} {split($3,r,"/"); if ($4!="Running" || r[1]!=r[2]) {print; n++}} END{if (!n) print "All pods are running and ready."}'`
}

// k8sRestarts lists the pods with most restarts, in every namespace.
func k8sRestarts(ctx string) string {
	return `O=$(` + kc(ctx, "get", "pods", "-A") + `) || exit 1
echo "$O" | head -1; echo "$O" | tail -n +2 | sort -k5 -rn | head -20`
}

func k8sWarnings(ctx string) string {
	return kc(ctx, "get", "events", "-A", "--field-selector", "type=Warning", "--sort-by=.lastTimestamp") + " | tail -n 40"
}

// k8sEmptyNamespaces offers the namespaces that have pods, when the selected one has none.
func (m *model) k8sEmptyNamespaces(ctx string) ([]list.Item, error) {
	s, err := out(nil, "kubectl", kcArgs(ctx, "get", "pods", "-A", "--no-headers", "-o", "custom-columns=NS:.metadata.namespace")...)
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, ns := range strings.Fields(s) {
		count[ns]++
	}
	names := make([]string, 0, len(count))
	for ns := range count {
		names = append(names, ns)
	}
	sort.Slice(names, func(i, j int) bool { return count[names[i]] > count[names[j]] })
	var items []list.Item
	for _, ns := range names {
		items = append(items, item{title: ns, desc: strconv.Itoa(count[ns]) + " pods", icon: iconK8s, color: cK8s,
			act: func(m *model) tea.Cmd {
				m.kubeNS = ns
				m.stack = m.stack[:len(m.stack)-1]
				return m.k8sPods()
			}})
	}
	return items, nil
}

func kcArgs(ctx string, args ...string) []string {
	if ctx != "" {
		return append([]string{"--context", ctx}, args...)
	}
	return args
}
