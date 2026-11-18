package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/albertovincenzi/bignami/internal/scripts"
	"github.com/albertovincenzi/bignami/internal/shell"
)

// A domain (AWS, Kubernetes) holds products (S3, SQS, Pods...), and a product holds commands.
type product struct {
	name, desc, icon string
	color            lipgloss.TerminalColor
	items            func() []list.Item
}

type domain struct {
	name     string
	icon     string
	color    lipgloss.TerminalColor
	blurb    string
	products []product
}

var domains = []domain{
	{name: "AWS", icon: iconAWS, color: cAWS, products: []product{sessionProduct, s3Product, sesProduct, snsProduct, sqsProduct, iamProduct}},
	{name: "Kubernetes", icon: iconK8s, color: cK8s, products: []product{k8sClusterProduct, k8sWorkloadsProduct, k8sDebugProduct}},
	{name: "GitHub", icon: "⎇", color: cGit, blurb: "PRs, CI runs", products: []product{ghPRProduct, ghActionsProduct, ghRepoProduct}},
	{name: "Helm", icon: "⎈", color: cHelm, blurb: "releases", products: []product{helmReleasesProduct, helmChartsProduct}},
	{name: "Google Cloud", icon: "☁", color: cGCP, blurb: "GKE, secrets, logs", products: []product{gcAccountProduct, gcGKEProduct, gcSecretsProduct, gcLogsProduct}},
	{name: "Docker", icon: "▣", color: cDocker, blurb: "containers, images", products: []product{dockerContainersProduct, dockerImagesProduct, dockerComposeProduct}},
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

var k8sClusterProduct = product{name: "Cluster", desc: "context, namespace, nodes", icon: iconK8s, color: cK8s, items: k8sClusterItems}

var k8sWorkloadsProduct = product{name: "Workloads", desc: "pods, unhealthy, restarts, deployments, usage", icon: iconK8s, color: cK8s, items: k8sWorkloadItems}

var k8sDebugProduct = product{name: "Debug", desc: "warnings, events, port-forward, free command", icon: iconK8s, color: cK8s, items: k8sDebugItems}

// kview runs a namespaced command with the hint for an empty namespace.
func kview(args ...string) func(m *model) tea.Cmd {
	return func(m *model) tea.Cmd { return m.capture(nil, scripts.K8sInNS(m.kubeCtx, m.kubeNS, args...)) }
}

// kall runs a script that covers every namespace.
func kall(script func(ctx string) string) func(m *model) tea.Cmd {
	return func(m *model) tea.Cmd { return m.capture(nil, script(m.kubeCtx)) }
}

func kplain(args ...string) func(m *model) tea.Cmd {
	return func(m *model) tea.Cmd { return m.capture(nil, scripts.Kc(m.kubeCtx, args...)) }
}

func k8sClusterItems() []list.Item {
	return []list.Item{
		k8s("Switch context", "only for this TUI, kubeconfig is not changed", func(m *model) tea.Cmd { return m.k8sContexts() }),
		k8s("Switch namespace", "the namespace of every command below", func(m *model) tea.Cmd { return m.k8sNamespaces() }),
		k8s("Namespaces", "all of them, with age", kplain("get", "namespaces")),
		k8s("Nodes", "status, version, IP, size", kplain("get", "nodes", "-o", "wide")),
		k8s("Node usage", "CPU and memory per node", kplain("top", "nodes")),
	}
}

func k8sWorkloadItems() []list.Item {
	return []list.Item{
		k8s("Pods", "pick one: logs, shell, describe. An empty namespace offers the full ones", func(m *model) tea.Cmd { return m.k8sPods() }),
		k8s("Unhealthy pods", "every namespace: not running or not ready", kall(scripts.K8sUnhealthy)),
		k8s("Most restarts", "every namespace, the top 20", kall(scripts.K8sRestarts)),
		k8s("Pods with node and IP", "this namespace, wide", kview("get", "pods", "-o", "wide")),
		k8s("Deployments", "this namespace: ready, up to date, image", kview("get", "deployments", "-o", "wide")),
		k8s("Deployments, all namespaces", "ready and age", kplain("get", "deployments", "-A")),
		k8s("Pod usage", "CPU and memory, this namespace", kview("top", "pods")),
		k8s("Pod usage, all namespaces", "sorted by memory", kplain("top", "pods", "-A", "--sort-by=memory")),
	}
}

func k8sDebugItems() []list.Item {
	return []list.Item{
		k8s("Warnings", "every namespace, the last 40 warning events", kall(scripts.K8sWarnings)),
		k8s("Recent events", "this namespace, sorted by time", kview("get", "events", "--sort-by=.lastTimestamp")),
		k8s("Port-forward", "e.g. svc/name 8080:80", func(m *model) tea.Cmd {
			return m.prompt("kubectl port-forward (context "+m.kubeCtx+")", false, func(m *model, v string) tea.Cmd {
				if v == "" {
					return nil
				}
				return m.guardProd(func(m *model) tea.Cmd {
					return m.capture(nil, shell.Join("kubectl", m.kargs()...)+" port-forward "+v)
				})
			})
		}),
		k8s("Custom kubectl command", "kubectl --context <ctx> <args>", func(m *model) tea.Cmd { return m.kubectlPrompt() }),
	}
}
