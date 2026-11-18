package scripts

import (
	"strings"
)

// After shows only what follows the last sep, for example the name at the end of an ARN.
func After(sep string) func(string) string {
	return func(s string) string {
		if i := strings.LastIndex(s, sep); i >= 0 {
			return s[i+len(sep):]
		}
		return s
	}
}
