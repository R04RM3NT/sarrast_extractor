package tui

import (
	"log/slog"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sarrast/internal/config"
	"sarrast/internal/database"
)

// screen identifies the active TUI screen.
type screen int

const (
	screenExtractForm screen = iota
	screenExtractRunning
	screenExtractResults
	screenExtractError
	screenSettings
	screenQuit
)

// Model is the top-level Bubble Tea model.
type Model struct {
	cfg     config.Config
	log     *slog.Logger
	db      *database.DB
	spinner spinner.Model

	screen screen
	width  int
	height int

	// sidebar selection
	sidebarIdx int

	// extract flow
	extract extractState

	// settings flow
	settings settingsState
}

// Message types.
type (
	// streamClosedMsg is emitted when a worker goroutine's channel closes.
	streamClosedMsg struct{}
)

// openDB is a test seam: production uses the centralized database.New, tests
// swap it for database.Open against a temporary file so they never touch
// data/series.db.
var openDB func(log *slog.Logger) (*database.DB, error) = database.New

// New builds the root model. The centralized database is opened once and
// shared with every component; there is no database selection anywhere.
func New(cfg config.Config, log *slog.Logger) (Model, error) {
	db, err := openDB(log)
	if err != nil {
		return Model{}, err
	}
	return Model{
		cfg: cfg,
		log: log,
		db:  db,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(purple)),
		),
		screen:     screenExtractForm,
		sidebarIdx: 0,
		extract:    newExtractState(cfg),
		settings:   newSettingsState(cfg.Proxy),
	}, nil
}

// Close releases the database handle owned by the model. It is called by tests
// (and on program exit through the model's teardown if one is added) so that a
// temporary database file can be removed.
func (m Model) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return m.spinner.Tick() }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Arrow keys drive the left sidebar whenever the current screen is not a
	// transient flow (running / results keep their own key handling).
	if km, ok := msg.(tea.KeyMsg); ok && m.sidebarNavigable() {
		switch km.String() {
		case "up":
			m.selectSection((m.sidebarIdx - 1 + len(navLabels)) % len(navLabels))
			return m, nil
		case "down":
			m.selectSection((m.sidebarIdx + 1) % len(navLabels))
			return m, nil
		}
	}

	switch m.screen {
	case screenExtractForm:
		return m.updateExtractForm(msg)
	case screenExtractRunning:
		return m.updateExtractRunning(msg)
	case screenExtractResults:
		return m.updateExtractResults(msg)
	case screenExtractError:
		return m.updateExtractError(msg)
	case screenSettings:
		return m.updateSettings(msg)
	case screenQuit:
		return m.updateQuit(msg)
	}
	return m, nil
}

func (m Model) View() string {
	var title, body, footer string
	switch m.screen {
	case screenExtractForm:
		title, body, footer = "EXTRACT LINKS", m.extractFormBody(), "tab / shift+tab navigate · enter run · ↑↓ switch section"
	case screenExtractRunning:
		title, body, footer = "EXTRACTING", m.extractRunningBody(), "ctrl+c cancel"
	case screenExtractResults:
		title, body, footer = "SERIES DATABASE", m.extractResultsBody(), "↑/↓ scroll · b back"
	case screenExtractError:
		title, body, footer = "EXTRACTION FAILED", m.extractErrorBody(), "enter / esc back · ↑↓ switch section"
	case screenSettings:
		title, body, footer = "SETTINGS", m.settingsBody(), "enter save · ↑↓ switch section"
	case screenQuit:
		title, body, footer = "QUIT", m.quitBody(), "enter quit · esc back"
	}
	return renderScreen(m.sidebarIdx, title, body, footer, m.width, m.height)
}

// ---- sidebar ----

// sidebarNavigable reports whether the sidebar can be navigated on the current
// screen (transient flows keep their own key handling).
func (m Model) sidebarNavigable() bool {
	switch m.screen {
	case screenExtractForm, screenExtractError, screenSettings, screenQuit:
		return true
	}
	return false
}

// selectSection switches to the section at the given sidebar index.
func (m *Model) selectSection(idx int) {
	m.sidebarIdx = idx
	switch idx {
	case 0:
		if m.screen == screenSettings || m.screen == screenQuit {
			m.screen = screenExtractForm
		}
	case 1:
		m.screen = screenSettings
	case 2:
		m.screen = screenQuit
	}
}

func (m Model) updateQuit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter":
			return m, tea.Quit
		case "esc":
			m.selectSection(0)
			return m, nil
		}
	}
	return m, nil
}

func (m Model) quitBody() string {
	return panel(
		warnStyle.Bold(true).Render("◆ QUIT?")+"\n\n"+dimStyle.Render("press enter to quit · esc to go back"),
		m.contentWidth()-8, true)
}

// listenStream returns one message from the channel; the caller re-subscribes
// to keep draining.
func listenStream(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return msg
	}
}

// newInput builds a styled text input.
func newInput(placeholder, value string, width int) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.SetValue(value)
	t.Prompt = "❯ "
	t.PromptStyle = lipgloss.NewStyle().Foreground(purple).Bold(true)
	t.Cursor.Style = lipgloss.NewStyle().Foreground(purple)
	t.TextStyle = lipgloss.NewStyle().Foreground(fg)
	t.PlaceholderStyle = lipgloss.NewStyle().Foreground(fgFaint).Italic(true)
	t.CharLimit = 500
	if width > 0 {
		t.Width = width
	}
	return t
}

// fieldView renders a labelled input row.
func fieldView(label string, input textinput.Model, focused bool) string {
	var l string
	if focused {
		l = labelFocus.Render(label)
	} else {
		l = labelStyle.Render(label)
	}
	return l + "\n" + input.View()
}
