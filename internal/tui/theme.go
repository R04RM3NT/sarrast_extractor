// Package tui implements the modern dark/cyberpunk themed terminal interface
// using Bubble Tea and Lip Gloss.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Palette. Deep black backgrounds with a bright purple primary accent.
var (
	// bg is the overall screen background.
	bg = lipgloss.Color("#07070d")
	// panelBg is used inside rounded panels.
	panelBg = lipgloss.Color("#0f0f18")
	// panelBgAlt is a slightly lifted variant for headers and bars.
	panelBgAlt = lipgloss.Color("#171727")
	// selectionBg is the bright purple used for selected/active elements.
	selectionBg = lipgloss.Color("#b14aff")
	// selectionBgDark backs glow rings.
	selectionBgDark = lipgloss.Color("#5b21b6")

	purple      = lipgloss.Color("#c97bff")
	purpleDim   = lipgloss.Color("#7c4dff")
	purpleFaint = lipgloss.Color("#3b2d63")
	fg          = lipgloss.Color("#e8e8f4")
	fgDim       = lipgloss.Color("#8f8fab")
	fgFaint     = lipgloss.Color("#5a5a78")

	green  = lipgloss.Color("#3ddc84")
	yellow = lipgloss.Color("#f7c948")
	red    = lipgloss.Color("#ff5c6c")
)

// Styles.
var (
	// screenStyle is applied to the full terminal view.
	screenStyle = lipgloss.NewStyle().Background(bg)

	headerBar = lipgloss.NewStyle().
			Background(selectionBgDark).
			Foreground(purple).
			Bold(true).
			Align(lipgloss.Center)

	labelStyle  = lipgloss.NewStyle().Foreground(fgDim)
	labelFocus  = lipgloss.NewStyle().Foreground(purple).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(fgFaint)
	mutedStyle  = lipgloss.NewStyle().Foreground(fgDim)
	okStyle     = lipgloss.NewStyle().Foreground(green)
	warnStyle   = lipgloss.NewStyle().Foreground(yellow)
	errStyle    = lipgloss.NewStyle().Foreground(red).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(purple)

	badgeStyle = lipgloss.NewStyle().
			Background(selectionBg).
			Foreground(lipgloss.Color("#0a0a12")).
			Bold(true).
			Padding(0, 1)

	badgeGreen = lipgloss.NewStyle().
			Background(lipgloss.Color("#14532d")).
			Foreground(green).
			Bold(true).
			Padding(0, 1)

	badgeRed = lipgloss.NewStyle().
			Background(lipgloss.Color("#7f1d2d")).
			Foreground(red).
			Bold(true).
			Padding(0, 1)

	badgeYellow = lipgloss.NewStyle().
			Background(lipgloss.Color("#713f12")).
			Foreground(yellow).
			Bold(true).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Background(panelBgAlt).
			Foreground(fgFaint)

	sidebarHeading = lipgloss.NewStyle().
			Background(selectionBgDark).
			Foreground(purple).
			Bold(true).
			Align(lipgloss.Center).
			Width(sidebarWidth)

	sidebarItemIdle = lipgloss.NewStyle().
			Foreground(fgDim).
			Padding(0, 2).
			Width(sidebarWidth)

	sidebarItemActive = lipgloss.NewStyle().
				Background(selectionBg).
				Foreground(lipgloss.Color("#0a0a12")).
				Bold(true).
				Padding(0, 2).
				Width(sidebarWidth)

	sidebarHint = lipgloss.NewStyle().
			Foreground(fgFaint).
			Padding(0, 2).
			Width(sidebarWidth)

	progressTrack = lipgloss.NewStyle().
			Background(panelBgAlt)
)

// sidebarWidth is the width of the left navigation column.
const sidebarWidth = 24

// navLabels lists the top-level sections shown in the left nav column.
var navLabels = []string{"EXTRACT LINKS", "SETTINGS", "QUIT"}

// sidebar renders the full-height left navigation column with the active
// section highlighted.
func sidebar(active int, h int) string {
	var b strings.Builder
	b.WriteString(sidebarHeading.Render("◆ NAVIGATION"))
	b.WriteString("\n\n")
	for i, label := range navLabels {
		item := "●  " + label
		if i == active {
			b.WriteString(sidebarItemActive.Render(item))
		} else {
			b.WriteString(sidebarItemIdle.Render(item))
		}
		b.WriteString("\n\n")
	}
	b.WriteString("\n")
	b.WriteString(sidebarHint.Render("↑ ↓ select section"))
	return lipgloss.NewStyle().
		Background(panelBgAlt).
		Width(sidebarWidth).
		Height(h).
		Render(b.String())
}

// progressGradient is the purple gradient used to fill progress bars.
var progressGradient = []string{"#4c1d95", "#6d28d9", "#8b5cf6", "#a855f7", "#c084fc", "#d8b4fe"}

// screen returns the full-screen view: a left navigation column plus the
// selected section (title bar, body filling the remaining space and a bottom
// hint bar) on the right.
func renderScreen(activeNav int, title string, body string, footer string, w, h int) string {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	side := sidebar(activeNav, h)
	rightW := w - sidebarWidth
	if rightW < 10 {
		rightW = 10
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, renderSection(title, body, footer, rightW, h))
}

// renderSection renders one section's title bar, full-height body and footer
// within the content pane.
func renderSection(title string, body string, footer string, w, h int) string {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	bar := headerBar.Width(w).Render("  ◆ " + title + "  ")
	foot := footerStyle.Width(w).Render(" " + footer + " ")

	centerH := h - 2
	if centerH < 1 {
		centerH = 1
	}
	center := screenStyle.Width(w).Height(centerH).Render(body)

	return bar + "\n" + center + "\n" + foot
}

// panel draws a rounded box with a neon purple border.
func panel(content string, width int, glow bool) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(purpleFaint).
		Background(panelBg).
		Padding(0, 2)

	if glow {
		style = style.BorderForeground(purple)
	}

	if width > 0 {
		style = style.Width(width)
	}
	return style.Render(content)
}

// progressBar renders an animated-style purple gradient bar.
func progressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	if width < 4 {
		width = 4
	}

	filled := int(percent * float64(width))
	// A moving highlight head makes the bar feel animated.
	head := filled
	if filled > 0 && filled < width {
		head = filled - 1
	}

	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			// Gradient across the filled region.
			g := progressGradient[(i*len(progressGradient))/width]
			if i == head {
				g = "#e879f9" // bright pulse at the leading edge
			}
			b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(g)).Render(" "))
			continue
		}
		b.WriteString(progressTrack.Render(" "))
	}

	return "▸ " + b.String() + " " + fmt.Sprintf("%3.0f%%", percent*100)
}

// stat renders a small labeled value used on the download progress screen.
func stat(label string, value string, valueStyle lipgloss.Style) string {
	return valueStyle.Bold(true).Render(value) + "  " + dimStyle.Render(label)
}

// formatDuration renders a duration as "5m 30s" or "-" when zero.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return d.String()
	}
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	s := (d % time.Minute) / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
