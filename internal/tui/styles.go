package tui

import "github.com/charmbracelet/lipgloss"

type styles struct{ header, tab, activeTab, panel, title, selected, muted, success, warning, danger, footer lipgloss.Style }

func midnight() styles {
	cyan, green, yellow, red, muted, surface := lipgloss.Color("#30DFE3"), lipgloss.Color("#70F25C"), lipgloss.Color("#FFC43D"), lipgloss.Color("#FF5F56"), lipgloss.Color("#78969C"), lipgloss.Color("#102C33")
	return styles{
		header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#061014")).Background(cyan).Padding(0, 1),
		tab:    lipgloss.NewStyle().Foreground(muted).Padding(0, 1), activeTab: lipgloss.NewStyle().Bold(true).Foreground(cyan).Padding(0, 1),
		panel: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cyan).Padding(0, 1), title: lipgloss.NewStyle().Bold(true).Foreground(cyan),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#061014")).Background(cyan), muted: lipgloss.NewStyle().Foreground(muted),
		success: lipgloss.NewStyle().Foreground(green), warning: lipgloss.NewStyle().Foreground(yellow), danger: lipgloss.NewStyle().Bold(true).Foreground(red),
		footer: lipgloss.NewStyle().Foreground(lipgloss.Color("#CDE4E7")).Background(surface).Padding(0, 1),
	}
}
