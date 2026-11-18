package tui

import tea "github.com/charmbracelet/bubbletea"

// Run starts the TUI and returns when the user quits.
func Run() error {
	m := newModel()
	m.detect()
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
