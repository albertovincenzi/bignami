package scripts

import (
	"github.com/albertovincenzi/bignami/internal/shell"
)

func DockerCmd(args ...string) string { return shell.Join("docker", args...) }

func DockerShellScript(name string) string {
	return DockerCmd("exec", "-it", name, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
}
