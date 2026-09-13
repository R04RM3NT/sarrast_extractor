package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sarrast/internal/config"
	"sarrast/internal/database"
	"sarrast/internal/scanner"
)

// Message types for the link-extraction flow.
type (
	progressMsg string
	runDoneMsg  struct {
		result scanner.Result
		err    error
	}
)

// extractState carries all state for the "Extract Links" flow.
type extractState struct {
	inputs     []textinput.Model
	focusIdx   int
	logs       []string
	stream     chan tea.Msg
	view       viewport.Model
	episodes   []*database.Episode
	seriesName string
	statusMsg  string
	errMsg     string
	lastScan   string
}

func newExtractState(cfg config.Config) extractState {
	inputs := []textinput.Model{
		newInput("https://sarrast.com/series/…", "", 0),  // Page URL
		newInput("My Series (optional)", "", 0),          // Series name
		newInput("http://… or socks5://…", cfg.Proxy, 0), // Proxy
		newInput("ouo.io", cfg.FilterHost, 0),            // Filter
	}
	inputs[0].Focus()

	view := viewport.New(0, 0)
	view.Style = lipgloss.NewStyle().Background(panelBg).Padding(0, 1)

	return extractState{inputs: inputs}
}

func (m *Model) extractFormBody() string {
	iw := m.inputWidth()
	for i := range m.extract.inputs {
		m.extract.inputs[i].Width = iw
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("extract \"s\" query params from shortener links") + "\n\n")

	labels := []string{"Page URL", "Series name (optional)", "Proxy (optional)", "Filter domain"}
	for i, input := range m.extract.inputs {
		b.WriteString(fieldView(labels[i], input, m.extract.focusIdx == i) + "\n\n")
	}

	// Read-only storage label: the database is fixed and internal.
	b.WriteString(okStyle.Render("◆ Storage: Central SQLite database"))
	b.WriteString("  " + dimStyle.Render(database.StoragePath) + "\n\n")

	if strings.TrimSpace(m.extract.inputs[0].Value()) != "" {
		b.WriteString(accentStyle.Bold(true).Background(selectionBg).Foreground(panelBg).Padding(0, 2).Render("  ▶ RUN  "))
	} else {
		b.WriteString(dimStyle.Render("  ▶ RUN  "))
	}

	return panel(b.String(), iw+8, true)
}

// contentWidth returns the width available to the section content pane (full
// width minus the sidebar).
func (m Model) contentWidth() int {
	w := m.width - sidebarWidth
	if w < 10 {
		w = 10
	}
	return w
}

func (m Model) inputWidth() int {
	w := m.contentWidth()
	if w == 0 {
		w = 80
	}
	iw := w - 16
	if iw < 24 {
		iw = 24
	}
	return iw
}

func (m Model) updateExtractForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for i := range m.extract.inputs {
			m.extract.inputs[i].Width = m.inputWidth()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.selectSection(2)
			return m, nil
		case "tab":
			m.focusNext()
			return m, nil
		case "shift+tab":
			m.focusPrev()
			return m, nil
		case "enter":
			if m.extract.focusIdx == len(m.extract.inputs)-1 {
				if strings.TrimSpace(m.extract.inputs[0].Value()) != "" {
					return m.startExtractRun()
				}
				m.focusInput(0)
			} else {
				m.focusNext()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.extract.inputs[m.extract.focusIdx], cmd = m.extract.inputs[m.extract.focusIdx].Update(msg)
	return m, cmd
}

func (m *Model) focusNext() {
	m.focusInput((m.extract.focusIdx + 1) % len(m.extract.inputs))
}

func (m *Model) focusPrev() {
	m.focusInput((m.extract.focusIdx - 1 + len(m.extract.inputs)) % len(m.extract.inputs))
}

func (m *Model) focusInput(idx int) {
	m.extract.inputs[m.extract.focusIdx].Blur()
	m.extract.focusIdx = idx
	m.extract.inputs[m.extract.focusIdx].Focus()
	if v := m.extract.inputs[m.extract.focusIdx].Value(); v != "" {
		m.extract.inputs[m.extract.focusIdx].CursorEnd()
	}
}

func (m Model) startExtractRun() (tea.Model, tea.Cmd) {
	m.screen = screenExtractRunning
	m.extract.logs = nil
	m.extract.stream = make(chan tea.Msg, 64)

	url := strings.TrimSpace(m.extract.inputs[0].Value())
	seriesName := strings.TrimSpace(m.extract.inputs[1].Value())
	proxy := strings.TrimSpace(m.extract.inputs[2].Value())
	filter := strings.TrimSpace(m.extract.inputs[3].Value())
	if filter == "" {
		filter = m.cfg.FilterHost
	}

	go runExtraction(m.cfg, m.db, url, seriesName, proxy, filter, m.extract.stream)
	return m, tea.Batch(listenStream(m.extract.stream), func() tea.Msg { return m.spinner.Tick() })
}

func (m Model) updateExtractRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progressMsg:
		m.extract.logs = appendLog(m.extract.logs, string(msg))
		return m, listenStream(m.extract.stream)
	case runDoneMsg:
		if msg.err != nil {
			m.extract.errMsg = msg.err.Error()
			m.screen = screenExtractError
			return m, nil
		}
		m.extract.episodes = msg.result.Series.Episodes()
		m.extract.seriesName = msg.result.Series.Name
		m.extract.lastScan = formatTime(msg.result.Series.LastScannedAt)
		m.extract.statusMsg = fmt.Sprintf("%d new, %d total episodes", len(msg.result.NewEpisodes), len(msg.result.Series.Episodes()))
		var content strings.Builder
		for i, episode := range msg.result.Series.Episodes() {
			content.WriteString(fmt.Sprintf("%4d  %s\n", i+1, episode.Link))
		}
		m.extract.view.SetContent(content.String())
		m.screen = screenExtractResults
		return m, nil
	case streamClosedMsg:
		return m, nil
	}
	return m, nil
}

func (m Model) extractRunningBody() string {
	boxW := m.contentWidth() - 8
	if boxW < 40 {
		boxW = 40
	}
	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}

	start := len(m.extract.logs) - maxLines
	if start < 0 {
		start = 0
	}

	var b strings.Builder
	b.WriteString(accentStyle.Render(m.spinner.View()) + "  working…\n\n")
	for _, l := range m.extract.logs[start:] {
		b.WriteString(mutedStyle.Render("  "+l) + "\n")
	}

	return panel(b.String(), boxW, true)
}

func (m Model) extractResultsBody() string {
	vw := m.contentWidth() - 8
	if vw < 30 {
		vw = 30
	}
	vh := m.height - 12
	if vh < 3 {
		vh = 3
	}
	m.extract.view.Width = vw - 6
	m.extract.view.Height = vh

	var b strings.Builder
	b.WriteString(accentStyle.Bold(true).Render("◆ SERIES · " + m.extract.seriesName))
	b.WriteString("  " + badgeStyle.Render(fmt.Sprintf("%d total", len(m.extract.episodes))) + "\n")
	b.WriteString(okStyle.Render("◆ Storage: Central SQLite database"))
	b.WriteString("  " + dimStyle.Render(database.StoragePath) + "\n")
	b.WriteString(dimStyle.Render("LAST SCAN · "+m.extract.lastScan) + "\n\n")
	b.WriteString(okStyle.Render(m.extract.statusMsg))
	b.WriteString("\n\n")
	b.WriteString(m.extract.view.View())

	return panel(b.String(), vw, true)
}

func (m Model) extractErrorBody() string {
	return panel(errStyle.Render("✗ "+m.extract.errMsg)+"\n\n"+dimStyle.Render("press enter to go back"), m.contentWidth()-8, true)
}

func (m Model) updateExtractResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "b", "esc":
			m.screen = screenExtractForm
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.extract.view, cmd = m.extract.view.Update(msg)
	return m, cmd
}

func (m Model) updateExtractError(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter", "esc", " ":
			m.screen = screenExtractForm
			return m, nil
		}
	}
	return m, nil
}

// appendLog keeps a bounded log buffer.
func appendLog(logs []string, line string) []string {
	const maxLogs = 500
	logs = append(logs, line)
	if len(logs) > maxLogs {
		logs = logs[len(logs)-maxLogs:]
	}
	return logs
}

// runExtraction performs the fetch + parse + merge in a goroutine, always
// against the centralized database owned by the model.
func runExtraction(cfg config.Config, db *database.DB, pageURL, seriesName, proxyURL, filterHost string, ch chan<- tea.Msg) {
	defer close(ch)

	result, err := scanner.Scan(db, pageURL, seriesName, proxyURL, filterHost, cfg.HTTPTimeout, func(message string) {
		ch <- progressMsg(message)
	})
	if err != nil {
		ch <- runDoneMsg{err: err}
		return
	}
	ch <- runDoneMsg{result: result}
}

// formatTime mirrors the web helper for a compact local timestamp.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}
