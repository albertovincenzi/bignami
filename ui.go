package main

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Palette: violet and lime on a quiet slate base. Adaptive, so it also reads on light terminals.
var (
	cText   = lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#F1F5F9"}
	cMuted  = lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}
	cBorder = lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"}
	cAccent = lipgloss.AdaptiveColor{Light: "#6741EA", Dark: "#A48BFF"}
	cBar    = lipgloss.AdaptiveColor{Light: "#6741EA", Dark: "#E4FF00"}
	cSelBg  = lipgloss.AdaptiveColor{Light: "#EDEFFF", Dark: "#241C52"}
	cOk     = lipgloss.Color("#22C55E")
	cWarn   = lipgloss.Color("#F59E0B")
	cErr    = lipgloss.Color("#EF4444")
	cAWS    = lipgloss.Color("#FF9900")
	cK8s    = lipgloss.AdaptiveColor{Light: "#326CE5", Dark: "#6C9BFF"}
)

const (
	iconAWS       = "◆"
	iconK8s       = "⬢"
	sessionWindow = 12 * time.Hour
)

func tint(c lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

func pill(bg lipgloss.TerminalColor, s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(bg).Padding(0, 1).Render(s)
}

func gauge(frac float64, width int, c lipgloss.TerminalColor) string {
	n := min(max(int(math.Round(frac*float64(width))), 0), width)
	return tint(c).Render(strings.Repeat("█", n)) + tint(cBorder).Render(strings.Repeat("░", width-n))
}

func fmtLeft(d time.Duration) string {
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// shortCtx turns gke_<project>_<region>_<cluster> into "<project> · <cluster>".
func shortCtx(c string) string {
	if p := strings.Split(c, "_"); len(p) == 4 && p[0] == "gke" {
		return p[1] + " · " + p[3]
	}
	return c
}

// --- list row ---

type delegate struct{}

func (delegate) Height() int                         { return 2 }
func (delegate) Spacing() int                        { return 1 }
func (delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (delegate) Render(w io.Writer, l list.Model, idx int, li list.Item) {
	it, ok := li.(item)
	if !ok {
		return
	}
	width := l.Width()
	icon := it.icon
	if icon == "" {
		icon = "›"
	}
	var ic lipgloss.TerminalColor = cAccent
	if it.color != nil {
		ic = it.color
	}
	title := ansi.Truncate(it.title, width-6, "…")
	desc := ansi.Truncate(it.desc, width-6, "…")

	if idx != l.Index() {
		fmt.Fprint(w, "  "+tint(ic).Render(icon)+" "+tint(cText).Render(title)+"\n    "+tint(cMuted).Render(desc))
		return
	}
	on := func(s lipgloss.Style) lipgloss.Style { return s.Background(cSelBg) }
	sp := on(lipgloss.NewStyle()).Render(" ")
	bar := on(tint(cBar)).Render("▌")
	row := lipgloss.NewStyle().Background(cSelBg).Width(width)
	l1 := bar + sp + on(tint(ic)).Render(icon) + sp + on(tint(cText).Bold(true)).Render(title)
	l2 := bar + sp + sp + sp + on(tint(cAccent)).Render(desc)
	fmt.Fprint(w, row.Render(l1)+"\n"+row.Render(l2))
}

func styleList(l *list.Model) {
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.Styles.Title = tint(cAccent).Bold(true)
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
	l.Styles.FilterPrompt = tint(cBar).Bold(true)
	l.Styles.FilterCursor = tint(cBar)
	l.Styles.ActivePaginationDot = tint(cAccent)
	l.Styles.InactivePaginationDot = tint(cBorder)
}

// --- header ---

var (
	cViolet = lipgloss.Color("#6741EA")
	cLime   = lipgloss.Color("#E4FF00")
	cWhite  = lipgloss.Color("#FFFFFF")
)

// logoMark is the bignami icon: a tired face with a sticky note: \"ls ?\", drawn with half blocks (3 rows, 6 columns).
func logoMark() [3]string {
	v := tint(cViolet)
	bg := lipgloss.NewStyle().Background(cViolet)
	return [3]string{
		v.Render("▄▄▄▄▄▄"),
		bg.Render(" ") + lipgloss.NewStyle().Background(cLime).Foreground(lipgloss.Color("#0F172A")).Bold(true).Render("?") + bg.Foreground(cWhite).Bold(true).Render("-_-") + bg.Render(" "),
		v.Render("▀▀▀▀▀▀"),
	}
}

// banner is the logo with the name and the tagline. Short terminals get a one-line version.
func (m *model) banner() string {
	name := tint(cText).Bold(true).Render("bignami")
	if version != "dev" {
		name += " " + tint(cMuted).Render(version)
	}
	tag := tint(cMuted).Render("a cheat sheet for forgetful devs")
	mark := logoMark()
	if m.h > 0 && m.h < 34 {
		return mark[1] + "  " + name + "  " + tag
	}
	return mark[0] + "\n" + mark[1] + "  " + name + "\n" + mark[2] + "  " + tag
}

func (m *model) head(cw int) string {
	banner := m.banner()

	each, gap := cw, 0
	if cw >= 76 {
		each, gap = (cw-1)/2, 1
	}
	cards := []string{m.awsCard(each)}
	if gap > 0 {
		cards = append(cards, " ")
	}
	if gap == 0 {
		cards = append(cards, m.k8sCard(cw))
	} else {
		cards = append(cards, m.k8sCard(cw-each-gap))
	}
	var row string
	if gap > 0 {
		row = lipgloss.JoinHorizontal(lipgloss.Top, cards...)
	} else {
		row = lipgloss.JoinVertical(lipgloss.Left, cards[0], cards[1])
	}
	return banner + "\n\n" + row
}

func card(width int, border lipgloss.TerminalColor, lines ...string) string {
	inner := max(width-4, 10)
	for i, ln := range lines {
		lines[i] = ansi.Truncate(ln, inner, "…")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
}

func (m *model) awsCard(width int) string {
	label := tint(cAWS).Bold(true).Render(iconAWS + " AWS")
	name := tint(cText).Bold(true).Render(m.awsProfile) + "  " + tint(cMuted).Render(m.regionLabel())
	session := tint(cMuted).Render("long-term credentials")
	border := lipgloss.TerminalColor(cBorder)
	if left, ok := awsRemaining(m.awsProfile); ok {
		switch {
		case left <= 0:
			session = tint(cErr).Bold(true).Render("session expired · run MFA login")
			border = cErr
		default:
			c := lipgloss.TerminalColor(cOk)
			if left < 30*time.Minute {
				c = cErr
			} else if left < 2*time.Hour {
				c = cWarn
			}
			session = gauge(float64(left)/float64(sessionWindow), 12, c) + " " + tint(c).Bold(true).Render(fmtLeft(left))
		}
	}
	return card(width, border, label, name, session)
}

func (m *model) k8sCard(width int) string {
	label := tint(cK8s).Bold(true).Render(iconK8s + " KUBERNETES")
	border := lipgloss.TerminalColor(cBorder)
	if isProd(m.kubeCtx) {
		label += "  " + pill(cErr, "PROD")
		border = cErr
	}
	ctx := m.kubeCtx
	if ctx == "" {
		ctx = "no context"
	}
	ns := m.kubeNS
	if ns == "" {
		ns = "default"
	}
	return card(width, border, label, tint(cText).Bold(true).Render(shortCtx(ctx)),
		tint(cMuted).Render("namespace ")+tint(cText).Render(ns))
}

// --- prompt and footer ---

func (m *model) promptView(w, h int) string {
	border := lipgloss.TerminalColor(cAccent)
	if strings.Contains(m.label, "type '") {
		border = cErr
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(1, 3).Width(min(w, 66) - 2).
		Render(tint(cText).Bold(true).Render(m.label) + "\n\n" + m.in.View())
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func hints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, tint(cText).Bold(true).Render(pairs[i])+" "+tint(cMuted).Render(pairs[i+1]))
	}
	return strings.Join(parts, tint(cBorder).Render("  ·  "))
}

func (m *model) foot(cw int) string {
	flat := func(s string) string {
		return ansi.Truncate(strings.Join(strings.Fields(s), " "), cw-6, "…")
	}
	status := " "
	switch {
	case m.busy != "":
		status = m.spin.View() + " " + tint(cMuted).Render(m.busy)
	case m.err != "":
		status = pill(cErr, "✗ "+flat(m.err))
	case m.info != "":
		status = pill(cOk, "✓ "+flat(m.info))
	}
	var h string
	switch {
	case m.prompting:
		h = hints("enter", "run", "esc", "cancel")
	case m.cur() == nil:
		h = hints("←→", "choose", "enter", "open", "1/2", "jump", "q", "quit")
	case m.focusOut:
		h = hints("↑↓ pgup pgdn", "scroll", "g/G", "top/bottom", "x", "stop", "tab", "back")
	case m.cur().FilterState() == list.Filtering:
		h = hints("enter", "apply", "esc", "clear")
	case m.running():
		h = hints("↑↓", "move", "enter", "run", "tab", "output", "x", "stop", "esc", "back", "q", "quit")
	default:
		keys := []string{"↑↓", "move", "enter", "run", "/", "filter", "tab", "output"}
		if m.sel == 0 {
			keys = append(keys, "r", "region")
		}
		h = hints(append(keys, "esc", "back", "q", "quit")...)
	}
	return status + "\n" + ansi.Truncate(h, cw, "…")
}

func (m *model) View() string {
	if m.w == 0 {
		return ""
	}
	cw := m.cw()
	head := m.head(cw)
	var body string
	switch {
	case m.prompting:
		body = m.promptView(cw, m.listH())
	case m.cur() == nil:
		body = m.pickerView(cw, m.listH())
	default:
		body = m.domainBody()
	}
	body = lipgloss.NewStyle().Height(m.listH()).Render(body)
	page := lipgloss.JoinVertical(lipgloss.Left, head, "", body, m.foot(cw))
	return lipgloss.NewStyle().Padding(1, 2).Render(page)
}

// --- logos ---

const awsLetters = ` ▄▀█ █ █ █ █▀▀
 █▀█ ▀▄▀▄▀ ▄▄█`

const awsSmile = ` ╰──────────╯▶`

const k8sWheel = `       ▄▄▄▄▄▄▄       
   ▄███▀  │  ▀███▄   
 ▐███ ╲   │   ╱ ███▌ 
 ████  ╲  │  ╱  ████ 
 ███ ──── ◉ ──── ███ 
 ████ ╱  ╱ ╲  ╲ ████ 
 ▐███   ╱ │ ╲   ███▌ 
  ▀███▄▄     ▄▄███▀  
     ▀▀▀▀▀▀▀▀▀▀▀     `

// paint colors every character of art: spokes in the spoke color, the rest in hull.
func paint(art string, hull, spoke lipgloss.TerminalColor, spokes string) string {
	var b strings.Builder
	for _, r := range art {
		switch {
		case r == '\n' || r == ' ':
			b.WriteRune(r)
		case strings.ContainsRune(spokes, r):
			b.WriteString(tint(spoke).Bold(true).Render(string(r)))
		default:
			b.WriteString(tint(hull).Render(string(r)))
		}
	}
	return b.String()
}

func logo(name string) string {
	var art string
	if name == "AWS" {
		art = lipgloss.JoinVertical(lipgloss.Left,
			paint(awsLetters, cText, cText, ""), tint(cAWS).Bold(true).Render(awsSmile))
	} else {
		art = paint(k8sWheel, cK8s, lipgloss.Color("#FFFFFF"), "│╲╱─◉")
	}
	return lipgloss.Place(23, 9, lipgloss.Center, lipgloss.Center, art)
}

func (m *model) pickerView(w, h int) string {
	each := (w - 2) / 2
	var cards []string
	for i, d := range domains {
		sel := i == m.sel
		border, b := lipgloss.TerminalColor(cBorder), lipgloss.RoundedBorder()
		if sel {
			border, b = cBar, lipgloss.ThickBorder()
		}
		title := tint(cText).Bold(true).Render(d.name)
		sub := tint(cMuted).Render(fmt.Sprintf("%d products", len(d.products)))
		cta := " "
		if sel {
			cta = tint(cBar).Bold(true).Render("enter ▸ open")
		}
		body := title + "\n" + sub + "\n" + cta
		if h >= 19 {
			body = logo(d.name) + "\n\n" + body
		}
		cards = append(cards, lipgloss.NewStyle().Border(b).BorderForeground(border).
			Padding(1, 2).Width(each-2).Align(lipgloss.Center).Render(body))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Center, cards[0], " ", cards[1])
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, row)
}
