package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRecentsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	r := loadRecents()
	r.set("Bucket", "my-bucket")
	if got := loadRecents().get("Bucket"); got != "my-bucket" {
		t.Errorf("got %q after reload", got)
	}
	if st, err := os.Stat(r.path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("recent file: %v %v", err, st)
	}
	var nilRecents *recents
	nilRecents.set("x", "y") // must not panic
	if nilRecents.get("x") != "" {
		t.Error("nil recents must return empty")
	}
	if !recallable("Object key", false) || recallable("MFA code", true) || recallable("aws", false) || recallable("kubectl (context x)", false) || recallable("x - type 'x' to confirm", false) {
		t.Error("recallable rules are wrong")
	}
}

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeLine(m *model, s string) {
	m.Update(key(s))
	m.Update(key("enter"))
}

func TestAskCollectsValuesInOrderAndCancelsOnEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel()
	m.w, m.h = 120, 40
	var got []string
	m.ask([]promptSpec{{label: "A"}, {label: "B", optional: true}, {label: "C"}}, func(m *model, v []string) tea.Cmd {
		got = v
		return nil
	})
	typeLine(m, "one")
	m.Update(key("enter")) // B left empty
	typeLine(m, "three")
	if strings.Join(got, "|") != "one||three" {
		t.Errorf("got %q", got)
	}

	got = nil
	m.ask([]promptSpec{{label: "A2"}}, func(*model, []string) tea.Cmd { got = []string{"ran"}; return nil })
	m.Update(key("enter"))
	if got != nil || !strings.Contains(m.err, "required") {
		t.Errorf("an empty required value must cancel: got=%v err=%q", got, m.err)
	}
}

func TestConfirmNameRunsOnlyOnExactMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel()
	m.w, m.h = 120, 40
	ran := 0
	next := func(*model) tea.Cmd { ran++; return nil }
	m.confirmName("This deletes", "queue-a", next)
	typeLine(m, "queue-b")
	if ran != 0 || m.err != "cancelled" {
		t.Errorf("wrong name must cancel: ran=%d err=%q", ran, m.err)
	}
	m.confirmName("This deletes", "queue-a", next)
	typeLine(m, "queue-a")
	if ran != 1 {
		t.Errorf("exact name must run once, ran=%d", ran)
	}
}

func TestConfirmPromptsAreNeverPrefilled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel()
	m.recents.set("x - type 'x' to confirm", "x")
	m.confirmName("x", "x", func(*model) tea.Cmd { return nil })
	if m.in.Value() != "" {
		t.Errorf("a confirmation must start empty, got %q", m.in.Value())
	}
}

func TestEveryProductHasRunnableItems(t *testing.T) {
	for _, d := range domains {
		if len(d.products) == 0 {
			t.Errorf("%s has no products", d.name)
		}
		for _, p := range d.products {
			items := p.items()
			if len(items) == 0 {
				t.Errorf("%s/%s is empty", d.name, p.name)
			}
			for _, it := range items {
				i := it.(item)
				if i.title == "" || i.act == nil {
					t.Errorf("%s/%s has an incomplete item: %+v", d.name, p.name, i.title)
				}
			}
		}
	}
}

func TestProductScreensFitTheWindow(t *testing.T) {
	t.Setenv("HOME", filepath.Join(t.TempDir()))
	for _, size := range [][2]int{{70, 24}, {110, 30}, {130, 40}, {200, 50}} {
		for di, d := range domains {
			for _, p := range d.products {
				m := newModel()
				m.w, m.h = size[0], size[1]
				m.sel = di
				m.openDomain()
				m.push(p.name, p.items())
				v := m.View()
				for i, ln := range strings.Split(v, "\n") {
					if w := lineWidth(ln); w > m.w {
						t.Errorf("%s/%s %v line %d is %d wide", d.name, p.name, size, i, w)
					}
				}
				if n := strings.Count(v, "\n") + 1; n > m.h {
					t.Errorf("%s/%s %v has %d lines, window is %d", d.name, p.name, size, n, m.h)
				}
			}
		}
	}
}

func TestRegionPattern(t *testing.T) {
	for in, ok := range map[string]bool{"eu-west-1": true, "us-gov-west-1": true, "europe": false, "EU-WEST-1": false} {
		if regionRe.MatchString(in) != ok {
			t.Errorf("regionRe(%q) = %v", in, !ok)
		}
	}
}
