package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sarrast/internal/settings"
)

// settingsState carries the Settings screen state.
type settingsState struct {
	proxyInput textinput.Model
	status     string
	statusOK   bool
}

func newSettingsState(proxy string) settingsState {
	s := settingsState{}
	s.proxyInput = newInput("http://127.0.0.1:8080  ·  socks5://127.0.0.1:1080", proxy, 0)
	s.proxyInput.Focus()
	s.status = "direct connections (no proxy)"
	s.statusOK = true
	return s
}

func (m Model) settingsBody() string {
	iw := m.inputWidth()
	m.settings.proxyInput.Width = iw

	var b strings.Builder
	b.WriteString(dimStyle.Render("default proxy — applied to every request") + "\n\n")
	b.WriteString(fieldView("Proxy (http / https / socks5)", m.settings.proxyInput, true) + "\n\n")

	if m.settings.statusOK {
		b.WriteString(okStyle.Render("✓ "+m.settings.status) + "\n")
	} else {
		b.WriteString(errStyle.Render("✗ "+m.settings.status) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("leave empty to disable the proxy") + "\n")
	b.WriteString(accentStyle.Bold(true).Background(selectionBg).Foreground(panelBg).Padding(0, 2).Render("  ▶ SAVE  "))

	return panel(b.String(), iw+8, true)
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.selectSection(2)
			return m, nil
		case "enter":
			return m.saveSettings()
		}
	}

	var cmd tea.Cmd
	m.settings.proxyInput, cmd = m.settings.proxyInput.Update(msg)
	return m, cmd
}

func (m Model) saveSettings() (tea.Model, tea.Cmd) {
	proxy := strings.TrimSpace(m.settings.proxyInput.Value())
	m.settings.status, m.settings.statusOK = validateProxy(proxy)
	if !m.settings.statusOK {
		return m, nil
	}

	s := settings.Settings{Proxy: proxy}
	if err := s.Save(); err != nil {
		m.settings.status = "could not save settings: " + err.Error()
		m.settings.statusOK = false
		return m, nil
	}

	m.cfg.Proxy = proxy
	m.settings.status = "saved — proxy applied to all requests"
	m.settings.statusOK = true
	return m, nil
}

// validateProxy accepts empty (no proxy) or http/https/socks5/socks5h URLs.
func validateProxy(proxy string) (string, bool) {
	if proxy == "" {
		return "direct connections (no proxy)", true
	}
	lower := strings.ToLower(proxy)
	for _, scheme := range []string{"http://", "https://", "socks5://", "socks5h://"} {
		if strings.HasPrefix(lower, scheme) && len(proxy) > len(scheme) {
			return "saved — proxy applied to all requests", true
		}
	}
	return "proxy must start with http://, https:// or socks5://", false
}
