package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gustavoohrodrigues/permguard/internal/audit"
	"github.com/gustavoohrodrigues/permguard/internal/config"
	"github.com/gustavoohrodrigues/permguard/internal/domain"
	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/i18n"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
)

type ModeChanger interface {
	Validate(domain.FileMetadata, int) error
	ApplyMode(domain.FileMetadata, os.FileMode, int) error
}

type AuditWriter interface {
	Append(domain.AuditRecord) error
}

type Dependencies struct {
	Catalog     *i18n.Catalog
	Inspector   filesystem.Inspector
	Privilege   domain.PrivilegeInfo
	Config      config.Config
	Changer     ModeChanger
	Audit       AuditWriter
	AllowWrites bool
	AuditPath   string
}
type loadedMsg struct {
	path    string
	entries []domain.DirectoryEntry
	err     error
}
type inputMode int

const (
	inputNone inputMode = iota
	inputPath
	inputSearch
	inputPermission
	inputConfirmation
)

type appliedMsg struct {
	metadata domain.FileMetadata
	err      error
}
type auditLoadedMsg struct {
	records []domain.AuditRecord
	err     error
}

type Model struct {
	deps                               Dependencies
	styles                             styles
	width, height, screen, cursor      int
	path, filter, status               string
	themeName, filterType, sortMode    string
	entries                            []domain.DirectoryEntry
	auditRecords                       []domain.AuditRecord
	selected                           *domain.FileMetadata
	input                              textinput.Model
	inputMode                          inputMode
	pendingMode                        os.FileMode
	pending                            *domain.FileMetadata
	loading, quitting, colors, unicode bool
}

func Run(deps Dependencies) error {
	model := NewModel(deps)
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func NewModel(deps Dependencies) Model {
	input := textinput.New()
	input.CharLimit = 4096
	input.Width = 70
	start := deps.Config.StartPath
	if start == "" {
		start = "."
	}
	absolute, _ := filepath.Abs(start)
	themeName := deps.Config.Display.Theme
	if _, ok := palettes[themeName]; !ok {
		themeName = "midnight"
	}
	colors := deps.Config.Display.ColorMode != "none" && deps.Config.Display.ColorMode != "sem_cor"
	unicode := deps.Config.Display.Unicode != "off" && deps.Config.Display.Unicode != "false"
	return Model{deps: deps, styles: theme(themeName, colors, unicode), themeName: themeName, colors: colors, unicode: unicode, filterType: "all", sortMode: "name", path: absolute, status: deps.Catalog.T("status.loading"), input: input, loading: true}
}

func (m Model) Init() tea.Cmd { return m.load(m.path) }

func (m Model) load(path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := m.deps.Inspector.ReadDir(path)
		return loadedMsg{path: path, entries: entries, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.inputMode != inputNone {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc":
				m.inputMode = inputNone
				m.input.Blur()
				return m, nil
			case "enter":
				return m.submitInput()
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = m.translateError(msg.err)
			return m, nil
		}
		m.path, m.entries, m.cursor, m.filter = msg.path, msg.entries, 0, ""
		if len(m.entries) > 0 {
			selected := m.entries[0].Metadata
			m.selected = &selected
		}
		m.status = m.deps.Catalog.T("status.ready")
	case appliedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = m.translateError(msg.err)
			return m, nil
		}
		m.selected = &msg.metadata
		m.pending = nil
		m.screen = 2
		m.status = m.deps.Catalog.T("status.permission_changed")
	case auditLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = m.translateError(msg.err)
		} else {
			m.auditRecords = msg.records
			m.status = m.deps.Catalog.T("status.audit_loaded", len(msg.records))
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "1":
		m.screen = 0
	case "2":
		m.screen = 1
	case "3", "p":
		m.screen = 2
	case "4":
		m.screen = 3
	case "5":
		m.screen, m.loading = 4, true
		return m, m.loadAudit()
	case "?", "h":
		m.screen = 6
	case "a":
		m.screen, m.loading = 4, true
		return m, m.loadAudit()
	case "t":
		m.themeName = nextTheme(m.themeName)
		m.styles = theme(m.themeName, m.colors, m.unicode)
		m.status = m.deps.Catalog.T("status.theme_changed", m.themeName)
	case "C":
		m.colors = !m.colors
		m.styles = theme(m.themeName, m.colors, m.unicode)
		m.status = m.deps.Catalog.T("status.colors_changed")
	case "u":
		m.unicode = !m.unicode
		m.styles = theme(m.themeName, m.colors, m.unicode)
		m.status = m.deps.Catalog.T("status.unicode_changed")
	case "f":
		m.cycleFilterType()
	case "s":
		m.cycleSort()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter":
		if m.screen == 5 && m.pending != nil {
			m.inputMode = inputConfirmation
			m.input.SetValue("")
			m.input.Placeholder = m.deps.Catalog.T("label.confirmation_input")
			m.input.Focus()
			return m, textinput.Blink
		}
		return m.openSelected()
	case "m":
		return m.startPermissionEdit()
	case "backspace":
		parent := filepath.Dir(m.path)
		if parent != m.path {
			m.loading = true
			return m, m.load(parent)
		}
	case "g":
		m.inputMode = inputPath
		m.input.SetValue(m.path)
		m.input.Placeholder = m.deps.Catalog.T("label.path_input")
		m.input.Focus()
		return m, textinput.Blink
	case "/":
		m.inputMode = inputSearch
		m.input.SetValue(m.filter)
		m.input.Placeholder = m.deps.Catalog.T("label.search_input")
		m.input.Focus()
		return m, textinput.Blink
	case "c":
		m.filter = ""
		m.cursor = 0
		m.status = m.deps.Catalog.T("status.filter_cleared")
	case "r":
		m.loading = true
		return m, m.load(m.path)
	case "left":
		if m.screen == 6 {
			m.screen = 4
		} else if m.screen > 0 && m.screen < 5 {
			m.screen--
		}
	case "right", "tab":
		if m.screen < 4 {
			m.screen++
		} else {
			m.screen = 0
		}
	case "esc":
		if m.screen == 5 {
			m.pending = nil
			m.screen = 2
			m.status = m.deps.Catalog.T("status.change_cancelled")
		} else {
			m.screen = 1
		}
	}
	return m, nil
}

func (m Model) loadAudit() tea.Cmd {
	return func() tea.Msg {
		records, err := audit.ReadRecent(m.deps.AuditPath, 200)
		return auditLoadedMsg{records: records, err: err}
	}
}

func (m *Model) cycleFilterType() {
	order := []string{"all", "directory", "file", "symlink"}
	for i, value := range order {
		if value == m.filterType {
			m.filterType = order[(i+1)%len(order)]
			break
		}
	}
	m.cursor = 0
	m.syncSelection()
	m.status = m.deps.Catalog.T("status.type_filter", m.deps.Catalog.T("filter."+m.filterType))
}

func (m *Model) cycleSort() {
	order := []string{"name", "size", "mode", "modified"}
	for i, value := range order {
		if value == m.sortMode {
			m.sortMode = order[(i+1)%len(order)]
			break
		}
	}
	m.status = m.deps.Catalog.T("status.sort_changed", m.deps.Catalog.T("sort."+m.sortMode))
	m.syncSelection()
}

func (m *Model) syncSelection() {
	visible := m.visibleEntries()
	if len(visible) == 0 {
		m.selected = nil
		return
	}
	if m.cursor >= len(visible) {
		m.cursor = len(visible) - 1
	}
	selected := visible[m.cursor].Metadata
	m.selected = &selected
}

func (m *Model) move(delta int) {
	visible := m.visibleEntries()
	if len(visible) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(visible) {
		m.cursor = len(visible) - 1
	}
	selected := visible[m.cursor].Metadata
	m.selected = &selected
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	visible := m.visibleEntries()
	if len(visible) == 0 || m.cursor >= len(visible) {
		return m, nil
	}
	item := visible[m.cursor].Metadata
	if item.Type == domain.TypeDirectory {
		m.loading = true
		return m, m.load(item.Path)
	}
	m.selected = &item
	m.screen = 2
	if item.IsSymlink {
		m.status = m.deps.Catalog.T("status.symlink_readonly")
	}
	return m, nil
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	value, mode := strings.TrimSpace(m.input.Value()), m.inputMode
	m.inputMode = inputNone
	m.input.Blur()
	if mode == inputPath {
		absolute, err := filepath.Abs(value)
		if err != nil {
			m.status = m.deps.Catalog.T("error.caminho_invalido")
			return m, nil
		}
		m.loading = true
		return m, m.load(absolute)
	}
	if mode == inputPermission {
		parsed, err := permissions.Parse(value)
		if err != nil {
			m.status = m.translateError(err)
			return m, nil
		}
		m.pendingMode = parsed
		metadata := *m.selected
		m.pending = &metadata
		m.screen = 5
		m.status = m.deps.Catalog.T("status.preview_ready")
		return m, nil
	}
	if mode == inputConfirmation {
		if value != m.deps.Config.Security.ConfirmationWord {
			m.status = m.deps.Catalog.T("error.confirmacao_invalida")
			return m, nil
		}
		m.loading = true
		return m, m.applyPending()
	}
	m.filter = value
	m.cursor = 0
	m.status = m.deps.Catalog.T("status.filtered")
	visible := m.visibleEntries()
	if len(visible) > 0 {
		selected := visible[0].Metadata
		m.selected = &selected
	}
	return m, nil
}

func (m Model) startPermissionEdit() (tea.Model, tea.Cmd) {
	if !m.deps.AllowWrites {
		m.status = m.deps.Catalog.T("error.escrita_nao_habilitada")
		return m, nil
	}
	if m.selected == nil || m.deps.Changer == nil {
		m.status = m.deps.Catalog.T("error.nenhum_item_selecionado")
		return m, nil
	}
	if err := m.deps.Changer.Validate(*m.selected, m.deps.Privilege.EffectiveUID); err != nil {
		m.status = m.translateError(err)
		return m, nil
	}
	m.inputMode = inputPermission
	m.input.SetValue(m.selected.Mode.NumericMode)
	m.input.Placeholder = m.deps.Catalog.T("label.permission_input")
	m.input.Focus()
	return m, textinput.Blink
}

func (m Model) applyPending() tea.Cmd {
	expected, mode := *m.pending, m.pendingMode
	return func() tea.Msg {
		err := m.deps.Changer.ApplyMode(expected, mode, m.deps.Privilege.EffectiveUID)
		result, errorText := "sucesso", ""
		if err != nil {
			result, errorText = "falha", err.Error()
		}
		record := domain.AuditRecord{Timestamp: time.Now(), OperatorUser: m.deps.Privilege.User, OperatorUID: m.deps.Privilege.EffectiveUID, TargetPath: expected.Path, TargetType: expected.Type, Operation: "chmod", PreviousMode: expected.Mode.NumericMode, NewMode: permissions.FromFileMode(mode).NumericMode, PreviousOwner: expected.Owner.Name, PreviousGroup: expected.Group.Name, Result: result, Error: errorText, SymlinkDetected: expected.IsSymlink}
		if m.deps.Audit != nil {
			if auditErr := m.deps.Audit.Append(record); auditErr != nil && err == nil {
				err = auditErr
			}
		}
		if err != nil {
			return appliedMsg{err: err}
		}
		updated, inspectErr := m.deps.Inspector.Inspect(expected.Path)
		return appliedMsg{metadata: updated, err: inspectErr}
	}
}

func (m Model) visibleEntries() []domain.DirectoryEntry {
	needle := strings.ToLower(m.filter)
	result := make([]domain.DirectoryEntry, 0)
	for _, item := range m.entries {
		if needle != "" && !strings.Contains(strings.ToLower(item.Metadata.Name), needle) {
			continue
		}
		if m.filterType == "directory" && item.Metadata.Type != domain.TypeDirectory || m.filterType == "file" && item.Metadata.Type != domain.TypeRegular || m.filterType == "symlink" && item.Metadata.Type != domain.TypeSymlink {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i].Metadata, result[j].Metadata
		switch m.sortMode {
		case "size":
			return a.Size < b.Size
		case "mode":
			return a.Mode.NumericMode < b.Mode.NumericMode
		case "modified":
			return a.ModifiedAt.Before(b.ModifiedAt)
		default:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
	return result
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	width := m.width
	if width < 60 {
		width = 60
	}
	height := m.height
	if height < 18 {
		height = 18
	}
	header := m.header(width)
	tabs := m.tabs(width)
	contentHeight := height - lipgloss.Height(header) - lipgloss.Height(tabs) - 2
	var body string
	switch m.screen {
	case 0:
		body = m.overview(contentHeight, width)
	case 1:
		body = m.browser(contentHeight, width)
	case 2:
		body = m.details(contentHeight, width)
	case 3:
		body = m.permissionView(contentHeight, width)
	case 4:
		body = m.auditView(contentHeight, width)
	case 5:
		body = m.changePreview(contentHeight, width)
	default:
		body = m.help(contentHeight, width)
	}
	footer := m.styles.footer.Width(width - 4).Render(m.deps.Catalog.T("footer.keys"))
	if m.inputMode != inputNone {
		prompt := m.deps.Catalog.T("label.search_input")
		switch m.inputMode {
		case inputPath:
			prompt = m.deps.Catalog.T("label.path_input")
		case inputPermission:
			prompt = m.deps.Catalog.T("label.permission_input")
		case inputConfirmation:
			prompt = m.deps.Catalog.T("label.confirmation_input")
		case inputNone, inputSearch:
			// O prompt padrão já representa a pesquisa local.
		}
		footer = m.styles.footer.Width(width - 4).Render(prompt + ": " + m.input.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
}

func (m Model) changePreview(height, width int) string {
	if m.pending == nil {
		return m.styles.panel.Width(width - 4).Height(height - 2).Render(m.deps.Catalog.T("label.none"))
	}
	newInfo := permissions.FromFileMode(m.pendingMode)
	lines := []string{
		m.styles.danger.Render(m.deps.Catalog.T("change.preview_title")), "",
		kv(m.deps.Catalog.T("label.current_path"), m.pending.Path),
		kv(m.deps.Catalog.T("label.type"), m.deps.Catalog.T("type."+string(m.pending.Type))),
		kv(m.deps.Catalog.T("change.current"), m.pending.Mode.NumericMode+" · "+m.pending.Mode.SymbolicMode),
		kv(m.deps.Catalog.T("change.proposed"), newInfo.NumericMode+" · "+newInfo.SymbolicMode), "",
		m.styles.warning.Render(m.deps.Catalog.T("change.warning")),
		m.deps.Catalog.T("change.confirm_hint"),
	}
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) header(width int) string {
	mode := m.deps.Catalog.T("privilege.mode." + m.deps.Privilege.Mode)
	left := fmt.Sprintf("PERMGUARD  %s: %s (UID %d)", m.deps.Catalog.T("label.user"), m.deps.Privilege.User, m.deps.Privilege.EffectiveUID)
	right := m.themeName + "  [? " + m.deps.Catalog.T("label.menu") + "]  " + m.deps.Catalog.T("label.mode") + ": " + mode
	if width < 100 {
		right = m.themeName + "  [?]"
	}
	if width < 72 {
		right = "[?]"
	}
	// Deixa uma coluna de folga: alguns terminais fazem wrap ao escrever
	// exatamente na última coluna disponível.
	spaces := width - lipgloss.Width(left) - lipgloss.Width(right) - 3
	if spaces < 1 {
		spaces = 1
	}
	return m.styles.header.Render(left + strings.Repeat(" ", spaces) + right)
}

func (m Model) tabs(width int) string {
	names := []string{"screen.overview", "screen.browser", "screen.details", "screen.permissions", "screen.audit"}
	parts := make([]string, len(names))
	for i, key := range names {
		label := fmt.Sprintf("%d:%s", i+1, m.deps.Catalog.T(key))
		if width < 90 {
			label = strconv.Itoa(i + 1)
		}
		style := m.styles.tab
		if i == m.screen {
			style = m.styles.activeTab
		}
		parts[i] = style.Render(label)
	}
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}

func (m Model) overview(height, width int) string {
	p := m.deps.Privilege
	notice := m.deps.Catalog.T("privilege.notice.read_only")
	if p.IsRoot {
		notice = m.deps.Catalog.T("privilege.notice.no_password")
	}
	selected := "—"
	if m.selected != nil {
		selected = m.selected.Name + " · " + m.selected.Mode.NumericMode + " · " + m.selected.Owner.Name + ":" + m.selected.Group.Name
	}
	text := m.styles.title.Render(m.deps.Catalog.T("screen.overview")) + "\n\n" +
		fmt.Sprintf("%s\n%s\n%s\n%s: %s\n%s: %d\n%s: %s\n\n%s",
			m.deps.Catalog.T("privilege.user", p.User, p.EffectiveUID), m.deps.Catalog.T("privilege.ids", p.RealUID, p.EffectiveUID, p.RealGID, p.EffectiveGID), m.deps.Catalog.T("privilege.groups", strings.Join(p.Groups, ", ")), m.deps.Catalog.T("label.current_path"), m.path, m.deps.Catalog.T("label.items"), len(m.entries), m.deps.Catalog.T("label.selected"), selected, notice)
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(text)
}

func (m Model) browser(height, width int) string {
	visible := m.visibleEntries()
	lines := []string{m.styles.title.Render(m.deps.Catalog.T("screen.browser") + ": " + m.path), m.styles.muted.Render(m.deps.Catalog.T("table.header"))}
	max := height - 5
	start := 0
	if m.cursor >= max {
		start = m.cursor - max + 1
	}
	for idx, item := range visible[start:min(start+max, len(visible))] {
		actual := start + idx
		md := item.Metadata
		marker := "  "
		if actual == m.cursor {
			marker = "▶ "
		}
		icon := typeIcon(md.Type)
		line := fmt.Sprintf("%s%-4s %-28s %-14s %-14s %-10s %8s", marker, icon, truncate(md.Name, 28), truncate(md.Owner.Name, 14), truncate(md.Group.Name, 14), md.Mode.NumericMode, humanSize(md.Size))
		if actual == m.cursor {
			line = m.styles.selected.Render(line)
		}
		lines = append(lines, line)
	}
	if len(visible) == 0 {
		lines = append(lines, m.styles.warning.Render(m.deps.Catalog.T("label.none")))
	}
	status := m.status
	if m.loading {
		status = m.deps.Catalog.T("status.loading")
	}
	lines = append(lines, "", m.styles.muted.Render(status+" · "+m.deps.Catalog.T("label.filter")+": "+emptyAs(m.filter, m.deps.Catalog.T("label.none"))+" · "+m.deps.Catalog.T("label.type")+": "+m.deps.Catalog.T("filter."+m.filterType)+" · "+m.deps.Catalog.T("label.sort")+": "+m.deps.Catalog.T("sort."+m.sortMode)))
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) auditView(height, width int) string {
	lines := []string{m.styles.title.Render(m.deps.Catalog.T("screen.audit")), m.styles.muted.Render(m.deps.AuditPath), "", m.styles.muted.Render(m.deps.Catalog.T("audit.header"))}
	maxRows := height - 6
	start := max(0, len(m.auditRecords)-maxRows)
	for _, record := range m.auditRecords[start:] {
		resultStyle := m.styles.success
		if record.Result != "sucesso" {
			resultStyle = m.styles.danger
		}
		line := fmt.Sprintf("%s  %-8s %-7s %4s→%-4s %s", record.Timestamp.Local().Format("02/01 15:04"), resultStyle.Render(record.Result), record.Operation, record.PreviousMode, record.NewMode, truncate(record.TargetPath, max(12, width-55)))
		lines = append(lines, line)
	}
	if len(m.auditRecords) == 0 && !m.loading {
		lines = append(lines, m.styles.muted.Render(m.deps.Catalog.T("audit.empty")))
	}
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) details(height, width int) string {
	if m.selected == nil {
		return m.styles.panel.Width(width - 4).Height(height - 2).Render(m.deps.Catalog.T("label.none"))
	}
	md := *m.selected
	lines := []string{m.styles.title.Render(m.deps.Catalog.T("screen.details") + ": " + md.Path), "", kv(m.deps.Catalog.T("label.type"), m.deps.Catalog.T("type."+string(md.Type))), kv(m.deps.Catalog.T("label.owner"), md.Owner.Name+" (UID "+md.Owner.UID+")"), kv(m.deps.Catalog.T("label.group"), md.Group.Name+" (GID "+md.Group.GID+")"), kv(m.deps.Catalog.T("label.permissions"), md.Mode.NumericMode+" · "+md.Mode.SymbolicMode), kv(m.deps.Catalog.T("label.size"), humanSize(md.Size)), kv(m.deps.Catalog.T("label.inode"), strconv.FormatUint(md.Inode, 10)), kv(m.deps.Catalog.T("label.modified"), formatTime(md.ModifiedAt)), kv(m.deps.Catalog.T("label.filesystem"), map[bool]string{true: m.deps.Catalog.T("label.read_only"), false: m.deps.Catalog.T("label.read_write")}[md.ReadOnlyFilesystem])}
	if md.IsSymlink {
		exists := m.deps.Catalog.T("label.no")
		if md.SymlinkTargetExists != nil && *md.SymlinkTargetExists {
			exists = m.deps.Catalog.T("label.yes")
		}
		lines = append(lines, "", m.styles.warning.Render(m.deps.Catalog.T("warning.symlink")), kv(m.deps.Catalog.T("label.target"), md.SymlinkTarget), kv(m.deps.Catalog.T("label.target_exists"), exists))
	}
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) permissionView(height, width int) string {
	if m.selected == nil {
		return m.styles.panel.Width(width - 4).Height(height - 2).Render(m.deps.Catalog.T("label.none"))
	}
	info := m.selected.Mode
	explanations := permissions.Explain(info, m.selected.Type == domain.TypeDirectory)
	lines := []string{m.styles.title.Render(m.deps.Catalog.T("screen.permissions") + ": " + m.selected.Path), "", kv(m.deps.Catalog.T("label.numeric"), info.NumericMode), kv(m.deps.Catalog.T("label.symbolic"), info.SymbolicMode), ""}
	for _, exp := range explanations {
		lines = append(lines, m.styles.success.Render(m.deps.Catalog.T("scope."+exp.Scope)+" · "+exp.Bits))
		for _, code := range exp.Codes {
			lines = append(lines, "  • "+m.deps.Catalog.T(code))
		}
	}
	if info.SUID {
		lines = append(lines, "", m.styles.danger.Render(m.deps.Catalog.T("warning.suid")), m.deps.Catalog.T("special.suid"))
	}
	if info.SGID {
		code := "special.sgid.file"
		if m.selected.Type == domain.TypeDirectory {
			code = "special.sgid.dir"
		}
		lines = append(lines, m.styles.warning.Render(m.deps.Catalog.T(code)))
	}
	if info.Sticky {
		lines = append(lines, m.styles.warning.Render(m.deps.Catalog.T("special.sticky")))
	}
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) help(height, width int) string {
	text := m.styles.title.Render(m.deps.Catalog.T("screen.help")) + "\n\n" + m.deps.Catalog.T("help.keys") + "\n\n" + m.styles.warning.Render(m.deps.Catalog.T("help.readonly")) + "\n\n" + m.deps.Catalog.T("privilege.notice.no_password")
	return m.styles.panel.Width(width - 4).Height(height - 2).Render(text)
}
func (m Model) translateError(err error) string {
	code, _, _ := strings.Cut(err.Error(), ": ")
	translated := m.deps.Catalog.T("error." + code)
	if strings.HasPrefix(translated, "[") {
		return err.Error()
	}
	return translated
}
func typeIcon(t domain.FileType) string {
	switch t {
	case domain.TypeDirectory:
		return "[D]"
	case domain.TypeSymlink:
		return "[L]"
	case domain.TypeRegular:
		return "[F]"
	case domain.TypeSocket:
		return "[S]"
	case domain.TypeFIFO:
		return "[P]"
	case domain.TypeBlock:
		return "[B]"
	case domain.TypeCharacter:
		return "[C]"
	case domain.TypeUnknown:
		return "[?]"
	default:
		return "[?]"
	}
}
func humanSize(size int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
func formatTime(value time.Time) string { return value.Format("02/01/2006 15:04:05 -07:00") }
func kv(key, value string) string       { return fmt.Sprintf("%-22s %s", key+":", value) }
func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-1]) + "…"
}
func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
