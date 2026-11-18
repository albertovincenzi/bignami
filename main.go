package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	title, desc string
	icon        string
	color       lipgloss.TerminalColor
	act         func(m *model) tea.Cmd
}

func (i item) FilterValue() string { return i.title + " " + i.desc }

type (
	pushMsg struct {
		title string
		items []list.Item
	}
	errMsg       struct{ err error }
	loginDoneMsg struct{ profile string }
)

type model struct {
	stack      []*list.Model // empty = domain picker
	sel        int           // selected domain on the picker
	w, h       int
	in         textinput.Model
	spin       spinner.Model
	vp         viewport.Model
	run        *run
	runSeq     int
	showPane   bool // narrow screens: the pane replaces the list
	focusOut   bool // keys go to the output pane
	prompting  bool
	label      string
	done       func(*model, string) tea.Cmd
	busy       string
	err, info  string
	awsProfile string
	awsRegion  string
	kubeCtx    string
	kubeNS     string
	recents    *recents
	remember   bool // the current prompt stores its answer
}

func newModel() *model {
	in := textinput.New()
	in.Prompt = "❯ "
	in.PromptStyle = tint(cBar)
	in.Width = 44
	profile := envOr("AWS_PROFILE", "default")
	return &model{
		in:         in,
		vp:         viewport.New(10, 5),
		spin:       spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(tint(cAccent))),
		awsProfile: profile,
		recents:    loadRecents(),
	}
}

func (m *model) cur() *list.Model {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

func (m *model) listH() int {
	return max(m.h-2-lipgloss.Height(m.head(max(m.w-4, 40)))-1-2, 5)
}

func (m *model) push(title string, items []list.Item) {
	if c := m.cur(); c != nil {
		title = c.Title + " › " + title
	}
	l := list.New(items, delegate{}, m.listW(), m.listH())
	l.Title = title
	styleList(&l)
	m.stack = append(m.stack, &l)
}

// load runs fn in the background and pushes the resulting screen.
func (m *model) load(title string, fn func() ([]list.Item, error)) tea.Cmd {
	m.busy = "loading " + title + "..."
	return func() tea.Msg {
		items, err := fn()
		if err != nil {
			return errMsg{err}
		}
		return pushMsg{title, items}
	}
}

func (m *model) Init() tea.Cmd { return m.spin.Tick }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case pushMsg:
		m.busy = ""
		m.push(msg.title, msg.items)
		return m, nil
	case lineMsg:
		return m, m.onLine(msg)
	case doneMsg:
		m.onDone(msg)
		return m, nil
	case errMsg:
		m.busy, m.err = "", msg.err.Error()
		return m, nil
	case loginDoneMsg:
		m.busy, m.awsProfile = "", msg.profile
		m.info = "logged in. In your shell run: export AWS_PROFILE=" + msg.profile
		return m, nil
	case tea.KeyMsg:
		return m.key(msg)
	}
	if l := m.cur(); l != nil {
		var cmd tea.Cmd
		*l, cmd = l.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err, m.info = "", ""
	if k.String() == "ctrl+c" {
		return m.quit()
	}
	if m.prompting {
		switch k.String() {
		case "enter":
			v, done := strings.TrimSpace(m.in.Value()), m.done
			if m.remember {
				m.recents.set(m.label, v)
			}
			m.prompting = false
			m.in.Reset()
			return m, done(m, v)
		case "esc":
			m.prompting = false
			m.in.Reset()
			return m, nil
		}
		var cmd tea.Cmd
		m.in, cmd = m.in.Update(k)
		return m, cmd
	}
	if m.busy != "" {
		return m, nil
	}
	l := m.cur()
	if l == nil {
		return m.pickerKey(k)
	}
	if m.focusOut {
		return m.outKey(k)
	}
	state := l.FilterState()
	switch k.String() {
	case "q":
		if state != list.Filtering {
			return m.quit()
		}
	case "r":
		if m.sel == 0 && state != list.Filtering && !strings.HasSuffix(l.Title, "regions") {
			return m, switchRegion(m)
		}
	case "tab":
		if m.paneVisible() {
			m.focusOut = true
			return m, nil
		}
	case "x":
		if state != list.Filtering && m.running() {
			m.stop()
			return m, nil
		}
	case "esc":
		if state == list.Unfiltered {
			m.stack = m.stack[:len(m.stack)-1]
			return m, nil
		}
	case "enter":
		if it, ok := l.SelectedItem().(item); ok && state != list.Filtering && it.act != nil {
			return m, it.act(m)
		}
	}
	var cmd tea.Cmd
	*l, cmd = l.Update(k)
	return m, cmd
}

func (m *model) pickerKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "esc":
		return m.quit()
	case "left", "right", "up", "down", "h", "l", "tab", "shift+tab":
		m.sel = (m.sel + 1) % len(domains)
	case "1", "2":
		m.sel = int(k.String()[0] - '1')
		return m, m.openDomain()
	case "enter":
		return m, m.openDomain()
	}
	return m, nil
}

func (m *model) openDomain() tea.Cmd {
	d := domains[m.sel]
	m.push(d.name, productItems(d))
	return nil
}

// version is set at release time with -ldflags; "go install" builds read it from the module info.
var version = "dev"

const usage = "bignami: a TUI cheat sheet for AWS and Kubernetes commands.\nRun it without arguments. Flags: -v, --version  -h, --help"

// detect reads the current Kubernetes context and AWS region from the machine.
func (m *model) detect() {
	ctx, _ := out(nil, "kubectl", "config", "current-context") // no kubectl or no context: empty, shown as "no context"
	m.kubeCtx = strings.TrimSpace(ctx)
	m.awsRegion = detectRegion(m.awsProfile)
}

func main() {
	if bi, ok := debug.ReadBuildInfo(); ok && version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = bi.Main.Version
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Println("bignami", version)
			return
		case "-h", "--help", "help":
			fmt.Println(usage)
			return
		}
	}
	m := newModel()
	m.detect()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
