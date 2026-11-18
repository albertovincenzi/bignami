package tui

import (
	"testing"
)

func TestEveryDomainHasProductsWithCommands(t *testing.T) {
	want := map[string]bool{"AWS": true, "Kubernetes": true, "GitHub": true, "Helm": true, "Google Cloud": true, "Docker": true}
	for _, d := range domains {
		delete(want, d.name)
		if d.icon == "" || d.color == nil {
			t.Errorf("%s needs an icon and a color", d.name)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing domains: %v", want)
	}
}
