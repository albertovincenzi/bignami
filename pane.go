package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	wideMin   = 110 // from this width the output pane sits on the right
	leftWidth = 44
	maxLines  = 5000
)

var errStopped = errors.New("stopped")

type (
	lineMsg struct {
		id   int
		text string
	}
	doneMsg struct {
		id  int
		err error
	}
)

// run is one captured command whose output streams into the pane.
type run struct {
	id      int
	title   string
	lines   []string // raw lines, capped
	wrapped []string // lines wrapped to the pane width, capped
	started time.Time
	elapsed time.Duration
	done    bool
	err     error
	ch      chan tea.Msg
	cancel  context.CancelFunc
}

func (m *model) cw() int    { return max(m.w-4, 10) }
func (m *model) wide() bool { return m.w >= wideMin }

func (m *model) listW() int {
	if m.wide() {
		return leftWidth
	}
	return m.cw()
}

func (m *model) paneW() int {
	if m.wide() {
		return m.cw() - leftWidth - 1
	}
	return m.cw()
}

// paneVisible: on wide screens the pane is always there; on narrow ones it replaces the list while a run is shown.
func (m *model) paneVisible() bool {
	return m.cur() != nil && (m.wide() || (m.showPane && m.run != nil))
}

func (m *model) running() bool { return m.run != nil && !m.run.done }

// layout sizes the lists and the output viewport for the current window.
func (m *model) layout() {
	for _, l := range m.stack {
		l.SetSize(m.listW(), m.listH())
	}
	m.vp.Width = max(m.paneW()-4, 10)
	m.vp.Height = max(m.listH()-5, 1)
	if m.run != nil {
		m.rewrap()
	}
}

func (m *model) rewrap() {
	r := m.run
	r.wrapped = r.wrapped[:0]
	for _, ln := range r.lines {
		r.wrapped = appendCapped(r.wrapped, m.wrapLine(ln))
	}
	m.refreshPane(true)
}

func (m *model) wrapLine(s string) []string {
	return strings.Split(ansi.Wrap(s, m.vp.Width, ""), "\n")
}

func appendCapped(dst, add []string) []string {
	dst = append(dst, add...)
	if len(dst) > maxLines {
		dst = slices.Clone(dst[len(dst)-maxLines:])
	}
	return dst
}

func (m *model) refreshPane(follow bool) {
	atBottom := m.vp.AtBottom()
	m.vp.SetContent(strings.Join(m.run.wrapped, "\n"))
	if follow && atBottom {
		m.vp.GotoBottom()
	}
}

func cleanLine(s string) string {
	if i := strings.LastIndex(s, "\r"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ReplaceAll(s, "\t", "    ")
}

func wait(ch chan tea.Msg) tea.Cmd { return func() tea.Msg { return <-ch } }

// capture runs a shell script without a terminal and streams its output into the pane.
// Pagers are off, so the output arrives as plain text.
func (m *model) capture(env []string, script string) tea.Cmd {
	m.stop()
	m.runSeq++
	id := m.runSeq
	if env == nil {
		env = os.Environ()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Env = append(slices.Clone(env), "PAGER=cat", "AWS_PAGER=")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second

	ch := make(chan tea.Msg, 256)
	send := func(msg tea.Msg) bool {
		select {
		case ch <- msg:
			return true
		case <-ctx.Done():
			return false
		}
	}
	go func() {
		pr, pw := io.Pipe()
		cmd.Stdout, cmd.Stderr = pw, pw
		errc := make(chan error, 1)
		go func() { err := cmd.Run(); pw.Close(); errc <- err }()
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			if !send(lineMsg{id, sc.Text()}) {
				break
			}
		}
		io.Copy(io.Discard, pr) // a too-long line must not block the writer
		send(doneMsg{id, <-errc})
	}()

	m.run = &run{id: id, title: strings.Join(strings.Fields(script), " "), started: time.Now(), ch: ch, cancel: cancel}
	m.vp.SetContent("")
	m.vp.GotoBottom()
	m.showPane = true
	m.focusOut = !m.wide()
	m.layout()
	return wait(ch)
}

// stop ends the running command, including its child processes.
func (m *model) stop() {
	r := m.run
	if r == nil || r.done {
		return
	}
	r.cancel()
	r.done, r.err, r.elapsed = true, errStopped, time.Since(r.started)
}

func (m *model) quit() (tea.Model, tea.Cmd) {
	m.stop()
	return m, tea.Quit
}

func (m *model) onLine(msg lineMsg) tea.Cmd {
	r := m.run
	if r == nil || msg.id != r.id || r.done {
		return nil
	}
	ln := cleanLine(msg.text)
	r.lines = append(r.lines, ln)
	if len(r.lines) > maxLines {
		r.lines = slices.Clone(r.lines[len(r.lines)-maxLines:])
	}
	r.wrapped = appendCapped(r.wrapped, m.wrapLine(ln))
	m.refreshPane(true)
	return wait(r.ch)
}

func (m *model) onDone(msg doneMsg) {
	r := m.run
	if r == nil || msg.id != r.id || r.done {
		return
	}
	r.done, r.err, r.elapsed = true, msg.err, time.Since(r.started)
}

// outKey handles keys while the output pane has the focus.
func (m *model) outKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "tab":
		m.focusOut = false
		if !m.wide() {
			m.showPane = false
		}
		return m, nil
	case "x":
		m.stop()
		return m, nil
	case "g", "home":
		m.vp.GotoTop()
		return m, nil
	case "G", "end":
		m.vp.GotoBottom()
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(k)
	return m, cmd
}

func (m *model) paneView() string {
	w, h := m.paneW(), m.listH()
	inner := w - 4
	border := lipgloss.TerminalColor(cBorder)
	if m.focusOut {
		border = cBar
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Width(w - 2).Height(h - 2)
	r := m.run
	if r == nil {
		hint := tint(cMuted).Render("Output appears here.\nPick a command and press enter.")
		return box.Render(lipgloss.Place(inner, h-2, lipgloss.Center, lipgloss.Center, hint))
	}
	title := tint(cText).Bold(true).Render(ansi.Truncate("$ "+r.title, inner, "…"))
	var status string
	switch {
	case !r.done:
		status = m.spin.View() + " " + tint(cMuted).Render("running "+secs(time.Since(r.started)))
	case errors.Is(r.err, errStopped):
		status = tint(cWarn).Bold(true).Render("■ stopped after " + secs(r.elapsed))
	case r.err != nil:
		status = tint(cErr).Bold(true).Render("✗ " + r.err.Error() + " · " + secs(r.elapsed))
	default:
		status = tint(cOk).Bold(true).Render("✓ done in " + secs(r.elapsed))
	}
	rule := tint(cBorder).Render(strings.Repeat("─", inner))
	return box.Render(title + "\n" + status + "\n" + rule + "\n" + m.vp.View())
}

func secs(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

// domainBody lays out the command list and the output pane.
func (m *model) domainBody() string {
	list := lipgloss.NewStyle().Width(m.listW()).Height(m.listH()).Render(m.cur().View())
	switch {
	case m.wide():
		return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", m.paneView())
	case m.paneVisible():
		return m.paneView()
	}
	return list
}
