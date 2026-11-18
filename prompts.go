package main

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type promptSpec struct {
	label            string
	secret, optional bool
}

// recallable: values that must never be pre-filled. Secrets, confirmations and free commands
// (a pre-filled command could be run again by a stray Enter).
func recallable(label string, secret bool) bool {
	return !secret && !strings.Contains(label, "type '") &&
		!strings.HasPrefix(label, "aws") && !strings.HasPrefix(label, "kubectl")
}

func (m *model) prompt(label string, secret bool, done func(*model, string) tea.Cmd) tea.Cmd {
	m.prompting, m.label, m.done = true, label, done
	m.remember = recallable(label, secret)
	m.in.EchoMode = textinput.EchoNormal
	if secret {
		m.in.EchoMode = textinput.EchoPassword
	}
	m.in.SetValue("")
	if m.remember {
		m.in.SetValue(m.recents.get(label))
		m.in.CursorEnd()
	}
	m.in.Focus()
	return textinput.Blink
}

// ask collects several values, one prompt after the other, then calls done.
// An empty answer to a required prompt cancels the whole flow.
func (m *model) ask(specs []promptSpec, done func(*model, []string) tea.Cmd) tea.Cmd {
	return m.askFrom(specs, nil, done)
}

func (m *model) askFrom(specs []promptSpec, got []string, done func(*model, []string) tea.Cmd) tea.Cmd {
	if len(got) == len(specs) {
		return done(m, got)
	}
	sp := specs[len(got)]
	return m.prompt(sp.label, sp.secret, func(m *model, v string) tea.Cmd {
		if v == "" && !sp.optional {
			m.err = "cancelled: " + sp.label + " is required"
			return nil
		}
		return m.askFrom(specs, append(slices.Clone(got), v), done)
	})
}

// confirmName asks to type a name before a risky action.
func (m *model) confirmName(what, name string, next func(*model) tea.Cmd) tea.Cmd {
	return m.prompt(what+" - type '"+name+"' to confirm", false, func(m *model, v string) tea.Cmd {
		if v != name {
			m.err = "cancelled"
			return nil
		}
		return next(m)
	})
}
