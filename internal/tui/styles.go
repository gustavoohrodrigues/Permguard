package tui

import "github.com/charmbracelet/lipgloss"

type styles struct{ header, tab, activeTab, panel, title, selected, muted, success, warning, danger, footer lipgloss.Style }
type palette struct{ accent, foreground, background, surface, muted, success, warning, danger string }

var themeOrder = []string{"midnight", "nord", "gruvbox-dark", "dracula", "high-contrast"}
var palettes = map[string]palette{
	"midnight":      {"#30DFE3", "#CDE4E7", "#061014", "#102C33", "#78969C", "#70F25C", "#FFC43D", "#FF5F56"},
	"nord":          {"#88C0D0", "#ECEFF4", "#2E3440", "#3B4252", "#7B88A1", "#A3BE8C", "#EBCB8B", "#BF616A"},
	"gruvbox-dark":  {"#FABD2F", "#EBDBB2", "#1D2021", "#3C3836", "#928374", "#B8BB26", "#FE8019", "#FB4934"},
	"dracula":       {"#BD93F9", "#F8F8F2", "#282A36", "#44475A", "#6272A4", "#50FA7B", "#F1FA8C", "#FF5555"},
	"high-contrast": {"#00FFFF", "#FFFFFF", "#000000", "#151515", "#BFBFBF", "#00FF00", "#FFFF00", "#FF4444"},
}

func theme(name string, colors, unicode bool) styles {
	p, ok := palettes[name]
	if !ok {
		p = palettes["midnight"]
	}
	border := lipgloss.RoundedBorder()
	if !unicode {
		border = lipgloss.NormalBorder()
	}
	if !colors {
		return styles{
			header: lipgloss.NewStyle().Bold(true).Padding(0, 1), tab: lipgloss.NewStyle().Padding(0, 1), activeTab: lipgloss.NewStyle().Bold(true).Underline(true).Padding(0, 1),
			panel: lipgloss.NewStyle().Border(border).Padding(0, 1), title: lipgloss.NewStyle().Bold(true), selected: lipgloss.NewStyle().Reverse(true).Bold(true),
			muted: lipgloss.NewStyle().Faint(true), success: lipgloss.NewStyle(), warning: lipgloss.NewStyle().Bold(true), danger: lipgloss.NewStyle().Bold(true).Underline(true), footer: lipgloss.NewStyle().Padding(0, 1),
		}
	}
	accent, foreground, background := lipgloss.Color(p.accent), lipgloss.Color(p.foreground), lipgloss.Color(p.background)
	return styles{
		header: lipgloss.NewStyle().Bold(true).Foreground(background).Background(accent).Padding(0, 1),
		tab:    lipgloss.NewStyle().Foreground(lipgloss.Color(p.muted)).Padding(0, 1), activeTab: lipgloss.NewStyle().Bold(true).Foreground(accent).Padding(0, 1),
		panel: lipgloss.NewStyle().Border(border).BorderForeground(accent).Foreground(foreground).Padding(0, 1), title: lipgloss.NewStyle().Bold(true).Foreground(accent),
		selected: lipgloss.NewStyle().Bold(true).Foreground(background).Background(accent), muted: lipgloss.NewStyle().Foreground(lipgloss.Color(p.muted)),
		success: lipgloss.NewStyle().Foreground(lipgloss.Color(p.success)), warning: lipgloss.NewStyle().Foreground(lipgloss.Color(p.warning)), danger: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.danger)),
		footer: lipgloss.NewStyle().Foreground(foreground).Background(lipgloss.Color(p.surface)).Padding(0, 1),
	}
}

func nextTheme(current string) string {
	for i, name := range themeOrder {
		if name == current {
			return themeOrder[(i+1)%len(themeOrder)]
		}
	}
	return themeOrder[0]
}
