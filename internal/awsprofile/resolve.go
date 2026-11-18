package awsprofile

import (
	"strings"

	"github.com/albertovincenzi/bignami/internal/shell"
)

// ResolveProfile prefers the MFA session of a profile when it is still valid, because accounts that enforce MFA deny everything without it.
func ResolveProfile(p string) string {
	if p == "" {
		p = shell.EnvOr("AWS_PROFILE", "default")
	}
	if !strings.HasSuffix(p, MFASuffix) {
		if left, ok := Remaining(p + MFASuffix); ok && left > 0 {
			return p + MFASuffix
		}
	}
	return p
}
