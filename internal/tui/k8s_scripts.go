package tui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertovincenzi/bignami/internal/scripts"
	"github.com/albertovincenzi/bignami/internal/shell"
)

// k8sEmptyNamespaces offers the namespaces that have pods, when the selected one has none.
func (m *model) k8sEmptyNamespaces(ctx string) ([]list.Item, error) {
	s, err := shell.Output(nil, "kubectl", scripts.KcArgs(ctx, "get", "pods", "-A", "--no-headers", "-o", "custom-columns=NS:.metadata.namespace")...)
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
