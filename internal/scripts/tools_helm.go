package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

// HelmCmd always carries the context and namespace, like kubectl in the TUI: the kubeconfig is never changed.
func HelmCmd(ctx, ns string, args ...string) string {
	var pre []string
	if ctx != "" {
		pre = append(pre, "--kube-context", ctx)
	}
	if ns != "" {
		pre = append(pre, "-n", ns)
	}
	return shell.Join("helm", append(pre, args...)...)
}
