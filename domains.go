package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A domain (AWS, Kubernetes) holds products (S3, SQS, Pods...), and a product holds commands.
type product struct {
	name, desc, icon string
	color            lipgloss.TerminalColor
	items            func() []list.Item
}

type domain struct {
	name     string
	products []product
}

var domains = []domain{
	{name: "AWS", products: []product{sessionProduct, s3Product, sesProduct, snsProduct, sqsProduct, iamProduct}},
	{name: "Kubernetes", products: []product{k8sClusterProduct, k8sWorkloadsProduct, k8sDebugProduct}},
}

// productItems lists the products of a domain; choosing one opens its commands.
func productItems(d domain) []list.Item {
	items := make([]list.Item, 0, len(d.products))
	for _, p := range d.products {
		items = append(items, item{title: p.name, desc: p.desc, icon: p.icon, color: p.color,
			act: func(m *model) tea.Cmd {
				m.push(p.name, p.items())
				return nil
			}})
	}
	return items
}

func k8s(title, desc string, act func(m *model) tea.Cmd) item {
	return item{title: title, desc: desc, icon: iconK8s, color: cK8s, act: act}
}

var (
	k8sClusterProduct   = product{name: "Cluster", desc: "context and namespace", icon: iconK8s, color: cK8s, items: k8sClusterItems}
	k8sWorkloadsProduct = product{name: "Workloads", desc: "pods, deployments, resource usage", icon: iconK8s, color: cK8s, items: k8sWorkloadItems}
	k8sDebugProduct     = product{name: "Debug", desc: "events, port-forward, free command", icon: iconK8s, color: cK8s, items: k8sDebugItems}
)

func kubectlView(args ...string) func(m *model) tea.Cmd {
	return func(m *model) tea.Cmd {
		return m.capture(nil, shJoin("kubectl", m.kargs(args...)...))
	}
}

func k8sClusterItems() []list.Item {
	return []list.Item{
		k8s("Switch context", "only for this TUI, kubeconfig is not changed", func(m *model) tea.Cmd { return m.k8sContexts() }),
		k8s("Switch namespace", "", func(m *model) tea.Cmd { return m.k8sNamespaces() }),
	}
}

func k8sWorkloadItems() []list.Item {
	return []list.Item{
		k8s("Pods", "logs, shell, describe", func(m *model) tea.Cmd { return m.k8sPods() }),
		k8s("Deployments", "get deployments", kubectlView("get", "deployments")),
		k8s("Top pods", "CPU and memory", kubectlView("top", "pods")),
	}
}

func k8sDebugItems() []list.Item {
	return []list.Item{
		k8s("Recent events", "sorted by time", kubectlView("get", "events", "--sort-by=.lastTimestamp")),
		k8s("Port-forward", "e.g. svc/name 8080:80", func(m *model) tea.Cmd {
			return m.prompt("kubectl port-forward (context "+m.kubeCtx+")", false, func(m *model, v string) tea.Cmd {
				if v == "" {
					return nil
				}
				return m.guardProd(func(m *model) tea.Cmd {
					return m.capture(nil, shJoin("kubectl", m.kargs()...)+" port-forward "+v)
				})
			})
		}),
		k8s("Custom kubectl command", "kubectl --context <ctx> <args>", func(m *model) tea.Cmd { return m.kubectlPrompt() }),
	}
}
