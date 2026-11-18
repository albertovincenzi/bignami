package scripts

import (
	"fmt"
	"regexp"
)

var DigitsRe = regexp.MustCompile(`^\d{1,12}$`)

// Digits checks a number that goes into a command (PR number, run id, revision).
func Digits(name, v string) (string, error) {
	if !DigitsRe.MatchString(v) {
		return "", fmt.Errorf("%s must be a number", name)
	}
	return v, nil
}
