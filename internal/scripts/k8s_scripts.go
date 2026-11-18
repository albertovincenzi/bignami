package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

// Kc builds a kubectl command that always carries the context, so the kubeconfig is never changed.
func Kc(ctx string, args ...string) string {
	if ctx != "" {
		args = append([]string{"--context", ctx}, args...)
	}
	return shell.Join("kubectl", args...)
}

// K8sUnhealthy lists the pods that are not ready or not running, in every namespace.
func K8sUnhealthy(ctx string) string {
	return `O=$(` + Kc(ctx, "get", "pods", "-A") + `) || exit 1
echo "$O" | awk 'NR==1{print; next} $4=="Completed"{next} {split($3,r,"/"); if ($4!="Running" || r[1]!=r[2]) {print; n++}} END{if (!n) print "All pods are running and ready."}'`
}

// K8sRestarts lists the pods with most restarts, in every namespace.
func K8sRestarts(ctx string) string {
	return `O=$(` + Kc(ctx, "get", "pods", "-A") + `) || exit 1
echo "$O" | head -1; echo "$O" | tail -n +2 | sort -k5 -rn | head -20`
}

func K8sWarnings(ctx string) string {
	return Kc(ctx, "get", "events", "-A", "--field-selector", "type=Warning", "--sort-by=.lastTimestamp") + " | tail -n 40"
}

func KcNS(ctx, ns string, args ...string) string {
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	return Kc(ctx, args...)
}

// k8sInNS runs a command in one namespace. When the namespace is empty it lists the namespaces that have pods,
// because "No resources found" alone tells nothing.
func K8sInNS(ctx, ns string, args ...string) string {
	hint := Kc(ctx, "get", "pods", "-A", "--no-headers", "-o", "custom-columns=NS:.metadata.namespace")
	return `O=$(` + KcNS(ctx, ns, args...) + ` 2>&1); S=$?; echo "$O"
case "$O" in *"No resources found"*)
  echo; echo "Nothing in this namespace. Namespaces that have pods (pods, name):"
  ` + hint + ` | sort | uniq -c | sort -rn
  echo; echo "Pick one with Cluster > Switch namespace.";;
esac
exit $S`
}

func KcArgs(ctx string, args ...string) []string {
	if ctx != "" {
		return append([]string{"--context", ctx}, args...)
	}
	return args
}
