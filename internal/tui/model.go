package tui

import (
	"fmt"
	"os"
	"os/exec"
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
	fileeditor "github.com/gustavoohrodrigues/permguard/internal/editor"
	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/i18n"
	"github.com/gustavoohrodrigues/permguard/internal/identity"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
)

type ModeChanger interface {
	Validate(domain.FileMetadata, int) error
	Revalidate(domain.FileMetadata) error
	ApplyMode(domain.FileMetadata, os.FileMode, int) error
	ValidateOwner(domain.FileMetadata, domain.PrivilegeInfo) error
	ValidateGroup(domain.FileMetadata, int, domain.PrivilegeInfo) error
	ApplyOwnership(domain.FileMetadata, *int, *int, domain.PrivilegeInfo) error
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
	usage   domain.DiskUsage
	err     error
}
type inputMode int

const (
	inputNone inputMode = iota
	inputPath
	inputSearch
	inputPermission
	inputOwner
	inputGroup
	inputConfirmation
)

type appliedMsg struct {
	metadata  domain.FileMetadata
	operation string
	err       error
}
type auditLoadedMsg struct {
	records []domain.AuditRecord
	err     error
}
type editorFinishedMsg struct {
	metadata domain.FileMetadata
	err      error
}

type Model struct {
	deps                               Dependencies
	styles                             styles
	width, height, screen, cursor      int
	explanationTopic                   int
	path, filter, status               string
	themeName, filterType, sortMode    string
	entries                            []domain.DirectoryEntry
	diskUsage                          domain.DiskUsage
	auditRecords                       []domain.AuditRecord
	selected                           *domain.FileMetadata
	input                              textinput.Model
	inputMode                          inputMode
	pendingMode                        os.FileMode
	pending                            *domain.FileMetadata
	pendingOwner, pendingGroup         *int
	pendingOwnerName, pendingGroupName string
	pendingOperation                   string
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
	return Model{deps: deps, styles: theme(themeName, colors, unicode), themeName: themeName, colors: colors, unicode: unicode, filterType: "all", sortMode: "size", path: absolute, status: deps.Catalog.T("status.loading"), input: input, loading: true}
}

func (m Model) Init() tea.Cmd { return m.load(m.path) }

func (m Model) load(path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := m.deps.Inspector.ReadDir(path)
		usage, _ := m.deps.Inspector.DiskUsage(path)
		return loadedMsg{path: path, entries: entries, usage: usage, err: err}
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
		m.path, m.entries, m.diskUsage, m.cursor, m.filter = msg.path, msg.entries, msg.usage, 0, ""
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
		m.status = m.deps.Catalog.T("status." + msg.operation + "_changed")
	case auditLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = m.translateError(msg.err)
		} else {
			m.auditRecords = msg.records
			m.status = m.deps.Catalog.T("status.audit_loaded", len(msg.records))
		}
	case editorFinishedMsg:
		m.loading = false
		m.pending = nil
		if msg.err != nil {
			m.status = m.translateError(msg.err)
		} else {
			m.selected = &msg.metadata
			m.screen = 2
			m.status = m.deps.Catalog.T("status.editor_closed")
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
		m.explanationTopic = 0
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
	case "e":
		if m.screen == 3 {
			m.explanationTopic = (m.explanationTopic + 1) % 3
			m.status = m.deps.Catalog.T("status.explanation_topic", m.explanationTopic+1, 3)
		}
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
	case "o":
		return m.startOwnershipEdit(true)
	case "G":
		return m.startOwnershipEdit(false)
	case "v":
		return m.startFileEdit()
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
		m.pendingOwner, m.pendingGroup = nil, nil
		m.pendingOwnerName, m.pendingGroupName = "", ""
		m.pendingOperation = "permission"
		metadata := *m.selected
		m.pending = &metadata
		m.screen = 5
		m.status = m.deps.Catalog.T("status.preview_ready")
		return m, nil
	}
	if mode == inputOwner {
		userIdentity, userID, err := identity.LookupUser(value)
		if err != nil {
			m.status = m.translateError(err)
			return m, nil
		}
		if err := m.deps.Changer.ValidateOwner(*m.selected, m.deps.Privilege); err != nil {
			m.status = m.translateError(err)
			return m, nil
		}
		m.pendingOwner, m.pendingGroup = &userID, nil
		m.pendingOwnerName, m.pendingGroupName = userIdentity.Name, ""
		metadata := *m.selected
		m.pending, m.pendingOperation, m.screen = &metadata, "owner", 5
		m.status = m.deps.Catalog.T("status.preview_ready")
		return m, nil
	}
	if mode == inputGroup {
		groupIdentity, groupID, err := identity.LookupGroup(value)
		if err != nil {
			m.status = m.translateError(err)
			return m, nil
		}
		if err := m.deps.Changer.ValidateGroup(*m.selected, groupID, m.deps.Privilege); err != nil {
			m.status = m.translateError(err)
			return m, nil
		}
		m.pendingOwner, m.pendingGroup = nil, &groupID
		m.pendingOwnerName, m.pendingGroupName = "", groupIdentity.Name
		metadata := *m.selected
		m.pending, m.pendingOperation, m.screen = &metadata, "group", 5
		m.status = m.deps.Catalog.T("status.preview_ready")
		return m, nil
	}
	if mode == inputConfirmation {
		if value != m.deps.Config.Security.ConfirmationWord {
			m.status = m.deps.Catalog.T("error.confirmacao_invalida")
			return m, nil
		}
		m.loading = true
		if m.pendingOperation == "edit" {
			return m, m.editFileCmd()
		}
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

func (m Model) startFileEdit() (tea.Model, tea.Cmd) {
	if !m.deps.AllowWrites {
		m.status = m.deps.Catalog.T("error.escrita_nao_habilitada")
		return m, nil
	}
	if m.selected == nil || m.deps.Changer == nil {
		m.status = m.deps.Catalog.T("error.nenhum_item_selecionado")
		return m, nil
	}
	if err := fileeditor.ValidateTarget(*m.selected); err != nil {
		m.status = m.translateError(err)
		return m, nil
	}
	if err := m.deps.Changer.Validate(*m.selected, m.deps.Privilege.EffectiveUID); err != nil {
		m.status = m.translateError(err)
		return m, nil
	}
	metadata := *m.selected
	m.pending = &metadata
	m.pendingOperation = "edit"
	m.screen = 5
	m.status = m.deps.Catalog.T("status.preview_ready")
	return m, nil
}

func (m Model) editFileCmd() tea.Cmd {
	expected := *m.pending
	if err := m.deps.Changer.Revalidate(expected); err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	binary, err := fileeditor.Resolve(m.deps.Config.Editor.Command)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	command := exec.Command(binary, "--", expected.Path) // #nosec G204 -- editor resolvido por allowlist e caminho enviado como argumento separado.
	return tea.ExecProcess(command, func(processErr error) tea.Msg {
		result, errorText := "sucesso", ""
		if processErr != nil {
			result, errorText = "indeterminado", processErr.Error()
		}
		updated := expected
		current, inspectErr := m.deps.Inspector.Inspect(expected.Path)
		if inspectErr == nil {
			updated = current
		}
		if inspectErr != nil && processErr == nil {
			processErr = fmt.Errorf("alteracao_concluida_reinspecao_falhou: %w", inspectErr)
		}
		record := domain.AuditRecord{Timestamp: time.Now(), OperatorUser: m.deps.Privilege.User, OperatorUID: m.deps.Privilege.EffectiveUID, TargetPath: expected.Path, TargetType: expected.Type, Operation: "editar", PreviousMode: expected.Mode.NumericMode, NewMode: updated.Mode.NumericMode, PreviousOwner: expected.Owner.Name, NewOwner: updated.Owner.Name, PreviousGroup: expected.Group.Name, NewGroup: updated.Group.Name, Result: result, Error: errorText, SymlinkDetected: expected.IsSymlink}
		if m.deps.Audit != nil {
			if auditErr := m.deps.Audit.Append(record); auditErr != nil && processErr == nil {
				processErr = auditErr
			}
		}
		return editorFinishedMsg{metadata: updated, err: processErr}
	})
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

func (m Model) startOwnershipEdit(owner bool) (tea.Model, tea.Cmd) {
	if !m.deps.AllowWrites {
		m.status = m.deps.Catalog.T("error.escrita_nao_habilitada")
		return m, nil
	}
	if m.selected == nil || m.deps.Changer == nil {
		m.status = m.deps.Catalog.T("error.nenhum_item_selecionado")
		return m, nil
	}
	if owner {
		if err := m.deps.Changer.ValidateOwner(*m.selected, m.deps.Privilege); err != nil {
			m.status = m.translateError(err)
			return m, nil
		}
		m.inputMode = inputOwner
		m.input.SetValue(m.selected.Owner.Name)
		m.input.Placeholder = m.deps.Catalog.T("label.owner_input")
	} else {
		m.inputMode = inputGroup
		m.input.SetValue(m.selected.Group.Name)
		m.input.Placeholder = m.deps.Catalog.T("label.group_input")
	}
	m.input.Focus()
	return m, textinput.Blink
}

func (m Model) applyPending() tea.Cmd {
	expected, mode := *m.pending, m.pendingMode
	ownerID, groupID := m.pendingOwner, m.pendingGroup
	ownerName, groupName, operation := m.pendingOwnerName, m.pendingGroupName, m.pendingOperation
	return func() tea.Msg {
		var err error
		switch operation {
		case "owner", "group":
			err = m.deps.Changer.ApplyOwnership(expected, ownerID, groupID, m.deps.Privilege)
		default:
			err = m.deps.Changer.ApplyMode(expected, mode, m.deps.Privilege.EffectiveUID)
		}
		result, errorText := "sucesso", ""
		if err != nil {
			result, errorText = "falha", err.Error()
		}
		updated := expected
		var inspectErr error
		if err == nil {
			updated, inspectErr = m.deps.Inspector.Inspect(expected.Path)
		}
		newMode, newOwner, newGroup, auditOperation := expected.Mode.NumericMode, expected.Owner.Name, expected.Group.Name, "chmod"
		switch operation {
		case "permission":
			newMode = permissions.FromFileMode(mode).NumericMode
		case "owner":
			newOwner, auditOperation = ownerName, "chown"
		case "group":
			newGroup, auditOperation = groupName, "chgrp"
		}
		if inspectErr == nil && err == nil {
			newMode, newOwner, newGroup = updated.Mode.NumericMode, updated.Owner.Name, updated.Group.Name
		}
		record := domain.AuditRecord{Timestamp: time.Now(), OperatorUser: m.deps.Privilege.User, OperatorUID: m.deps.Privilege.EffectiveUID, TargetPath: expected.Path, TargetType: expected.Type, Operation: auditOperation, PreviousMode: expected.Mode.NumericMode, NewMode: newMode, PreviousOwner: expected.Owner.Name, NewOwner: newOwner, PreviousGroup: expected.Group.Name, NewGroup: newGroup, Result: result, Error: errorText, SymlinkDetected: expected.IsSymlink}
		if m.deps.Audit != nil {
			if auditErr := m.deps.Audit.Append(record); auditErr != nil && err == nil {
				err = auditErr
			}
		}
		if err != nil {
			return appliedMsg{operation: operation, err: err}
		}
		if inspectErr != nil {
			return appliedMsg{operation: operation, err: fmt.Errorf("alteracao_concluida_reinspecao_falhou: %w", inspectErr)}
		}
		return appliedMsg{metadata: updated, operation: operation, err: inspectErr}
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
		aDirectory, bDirectory := a.Type == domain.TypeDirectory, b.Type == domain.TypeDirectory
		if aDirectory != bDirectory {
			return aDirectory
		}
		if aDirectory && bDirectory && m.sortMode == "size" {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		switch m.sortMode {
		case "size":
			return a.Size > b.Size
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
	footerKey := "footer.keys"
	switch m.screen {
	case 1:
		footerKey = "footer.browser"
	case 2:
		footerKey = "footer.details"
	case 3:
		footerKey = "footer.explanations"
	case 4:
		footerKey = "footer.audit"
	case 5:
		footerKey = "footer.preview"
	}
	footer := m.styles.footer.Width(width - 4).Render(m.deps.Catalog.T(footerKey))
	if m.inputMode != inputNone {
		prompt := m.deps.Catalog.T("label.search_input")
		switch m.inputMode {
		case inputPath:
			prompt = m.deps.Catalog.T("label.path_input")
		case inputPermission:
			prompt = m.deps.Catalog.T("label.permission_input")
		case inputOwner:
			prompt = m.deps.Catalog.T("label.owner_input")
		case inputGroup:
			prompt = m.deps.Catalog.T("label.group_input")
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
	title := m.deps.Catalog.T("change.preview_title")
	current := m.pending.Mode.NumericMode + " · " + m.pending.Mode.SymbolicMode
	proposed := permissions.FromFileMode(m.pendingMode).NumericMode + " · " + permissions.FromFileMode(m.pendingMode).SymbolicMode
	impact := m.deps.Catalog.T("change.impact.mode")
	switch m.pendingOperation {
	case "owner":
		title = m.deps.Catalog.T("change.preview_owner_title")
		current = m.pending.Owner.Name + " (UID " + m.pending.Owner.UID + ")"
		proposed = m.pendingOwnerName + " (UID " + strconv.Itoa(*m.pendingOwner) + ")"
		impact = m.deps.Catalog.T("change.impact.owner")
	case "group":
		title = m.deps.Catalog.T("change.preview_group_title")
		current = m.pending.Group.Name + " (GID " + m.pending.Group.GID + ")"
		proposed = m.pendingGroupName + " (GID " + strconv.Itoa(*m.pendingGroup) + ")"
		impact = m.deps.Catalog.T("change.impact.group")
	case "edit":
		title = m.deps.Catalog.T("change.preview_edit_title")
		current = m.deps.Catalog.T("change.file_current", humanSize(m.pending.Size), m.pending.Mode.NumericMode)
		proposed = m.deps.Catalog.T("change.editor_proposed", m.deps.Config.Editor.Command)
		impact = m.deps.Catalog.T("change.impact.edit")
	}
	lines := []string{
		m.styles.danger.Render(title), "",
		kv(m.deps.Catalog.T("label.current_path"), m.pending.Path),
		kv(m.deps.Catalog.T("label.type"), m.deps.Catalog.T("type."+string(m.pending.Type))),
		kv(m.deps.Catalog.T("change.current"), current),
		kv(m.deps.Catalog.T("change.proposed"), proposed), "",
		m.styles.warning.Render(impact),
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
	directories, files, links := 0, 0, 0
	var fileBytes, largest int64
	for _, item := range visible {
		switch item.Metadata.Type {
		case domain.TypeDirectory:
			directories++
		case domain.TypeRegular:
			files++
			fileBytes += item.Metadata.Size
			if item.Metadata.Size > largest {
				largest = item.Metadata.Size
			}
		case domain.TypeSymlink:
			links++
		case domain.TypeSocket, domain.TypeFIFO, domain.TypeBlock, domain.TypeCharacter, domain.TypeUnknown:
			// Tipos especiais aparecem na lista, mas não entram na soma dos arquivos comuns.
		}
	}
	usage := fmt.Sprintf("%s  %s  %.1f%%  %s", m.deps.Catalog.T("disk.usage"), diskBar(m.diskUsage.UsedPercent, 22, m.unicode), m.diskUsage.UsedPercent, m.deps.Catalog.T("disk.summary", humanSizeUnsigned(m.diskUsage.UsedBytes), humanSizeUnsigned(m.diskUsage.TotalBytes), humanSizeUnsigned(m.diskUsage.AvailableBytes)))
	heading := m.styles.title.Render(m.deps.Catalog.T("screen.browser")) + "  " + m.styles.muted.Render(m.path) + "\n" + usage + "\n" + m.styles.muted.Render(m.deps.Catalog.T("browser.summary", directories, files, links, humanSize(fileBytes)))
	listWidth := width
	wide := width >= 100
	if wide {
		listWidth = width * 63 / 100
	}
	nameWidth := max(14, listWidth-35)
	lines := []string{m.styles.muted.Render(m.deps.Catalog.T("browser.list_header"))}
	maxRows := height - lipgloss.Height(heading) - 5
	if maxRows < 2 {
		maxRows = 2
	}
	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	for idx, item := range visible[start:min(start+maxRows, len(visible))] {
		actual := start + idx
		md := item.Metadata
		marker := " "
		if actual == m.cursor {
			marker = ">"
			if m.unicode {
				marker = "▸"
			}
		}
		icon := typeIcon(md.Type)
		name, size, bar := md.Name, humanSize(md.Size), ""
		if md.Type == domain.TypeDirectory {
			name, size = md.Name+"/", m.deps.Catalog.T("label.directory")
		} else if md.Type == domain.TypeRegular && largest > 0 {
			bar = sizeBar(float64(md.Size)/float64(largest), 8, m.unicode)
		}
		line := fmt.Sprintf("%s %-3s %-*s %8s %-8s %s", marker, icon, nameWidth, truncate(name, nameWidth), size, md.Mode.NumericMode, bar)
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
	lines = append(lines, "", m.styles.muted.Render(status+" · "+m.deps.Catalog.T("label.filter")+": "+emptyAs(m.filter, m.deps.Catalog.T("label.none"))+" · "+m.deps.Catalog.T("label.sort")+": "+m.deps.Catalog.T("sort."+m.sortMode)))
	panelHeight := max(6, height-lipgloss.Height(heading)-2)
	if !wide {
		return heading + "\n" + m.styles.panel.Width(width-4).Height(panelHeight-2).Render(strings.Join(lines, "\n"))
	}
	leftOuter := listWidth
	rightOuter := width - leftOuter
	left := m.styles.panel.Width(leftOuter - 4).Height(panelHeight - 2).Render(strings.Join(lines, "\n"))
	right := m.styles.panel.Width(rightOuter - 4).Height(panelHeight - 2).Render(m.browserDetails())
	return heading + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) browserDetails() string {
	if m.selected == nil {
		return m.styles.muted.Render(m.deps.Catalog.T("browser.no_selection"))
	}
	md := *m.selected
	lines := []string{
		m.styles.title.Render(m.deps.Catalog.T("browser.selected")), "",
		m.styles.success.Render(typeIcon(md.Type) + " " + md.Name), "",
		kv(m.deps.Catalog.T("label.type"), m.deps.Catalog.T("type."+string(md.Type))),
		kv(m.deps.Catalog.T("label.size"), humanSize(md.Size)),
		kv(m.deps.Catalog.T("label.permissions"), md.Mode.NumericMode),
		kv(m.deps.Catalog.T("label.owner"), md.Owner.Name),
		kv(m.deps.Catalog.T("label.group"), md.Group.Name),
		kv(m.deps.Catalog.T("label.modified"), md.ModifiedAt.Format("02/01/06 15:04")), "",
		m.styles.muted.Render(m.deps.Catalog.T("browser.actions")),
	}
	return strings.Join(lines, "\n")
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
		change := record.PreviousMode + "→" + record.NewMode
		switch record.Operation {
		case "chown":
			change = record.PreviousOwner + "→" + record.NewOwner
		case "chgrp":
			change = record.PreviousGroup + "→" + record.NewGroup
		}
		line := fmt.Sprintf("%s  %-8s %-7s %-19s %s", record.Timestamp.Local().Format("02/01 15:04"), resultStyle.Render(record.Result), record.Operation, truncate(change, 19), truncate(record.TargetPath, max(12, width-65)))
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
	lines := []string{m.styles.title.Render(m.deps.Catalog.T("screen.permissions") + ": " + m.selected.Path), m.styles.muted.Render(m.deps.Catalog.T("explanation.navigation", m.explanationTopic+1, 3)), ""}
	switch m.explanationTopic {
	case 1:
		lines = append(lines,
			m.styles.success.Render(m.deps.Catalog.T("explanation.ownership.title")), "",
			kv(m.deps.Catalog.T("label.owner"), m.selected.Owner.Name+" (UID "+m.selected.Owner.UID+")"),
			kv(m.deps.Catalog.T("label.group"), m.selected.Group.Name+" (GID "+m.selected.Group.GID+")"), "",
			m.deps.Catalog.T("explanation.owner"),
			m.deps.Catalog.T("explanation.group"),
			m.deps.Catalog.T("explanation.others"), "",
			m.styles.warning.Render(m.deps.Catalog.T("explanation.operations.title")),
			m.deps.Catalog.T("explanation.chmod"),
			m.deps.Catalog.T("explanation.chown"),
			m.deps.Catalog.T("explanation.chgrp"),
			m.deps.Catalog.T("explanation.acl"), "",
			m.styles.danger.Render(m.deps.Catalog.T("explanation.availability")))
	case 2:
		lines = append(lines,
			m.styles.success.Render(m.deps.Catalog.T("explanation.examples.title")), "",
			m.deps.Catalog.T("explanation.example.mode"),
			m.styles.muted.Render(m.deps.Catalog.T("explanation.example.mode.command")), "",
			m.deps.Catalog.T("explanation.example.owner"),
			m.styles.muted.Render(m.deps.Catalog.T("explanation.example.owner.command")), "",
			m.deps.Catalog.T("explanation.example.group"),
			m.styles.muted.Render(m.deps.Catalog.T("explanation.example.group.command1")),
			m.styles.muted.Render(m.deps.Catalog.T("explanation.example.group.command2")), "",
			m.deps.Catalog.T("explanation.example.both"),
			m.styles.muted.Render(m.deps.Catalog.T("explanation.example.both.command")), "",
			m.deps.Catalog.T("explanation.example.shared"),
			m.styles.muted.Render(m.deps.Catalog.T("explanation.example.shared.command")), "",
			m.styles.warning.Render(m.deps.Catalog.T("explanation.privilege")),
			m.deps.Catalog.T("explanation.no_password"))
	default:
		explanations := permissions.Explain(info, m.selected.Type == domain.TypeDirectory)
		lines = append(lines, kv(m.deps.Catalog.T("label.numeric"), info.NumericMode), kv(m.deps.Catalog.T("label.symbolic"), info.SymbolicMode), "")
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
func humanSizeUnsigned(size uint64) string {
	if size > uint64(^uint64(0)>>1) {
		return fmt.Sprintf("%.1f EiB", float64(size)/(1<<60))
	}
	return humanSize(int64(size))
}
func diskBar(percent float64, width int, unicode bool) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent * float64(width) / 100)
	on, off := "#", "-"
	if unicode {
		on, off = "█", "░"
	}
	return "[" + strings.Repeat(on, filled) + strings.Repeat(off, width-filled) + "]"
}
func sizeBar(ratio float64, width int, unicode bool) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	if filled == 0 && ratio > 0 {
		filled = 1
	}
	on, off := "=", " "
	if unicode {
		on, off = "▰", "·"
	}
	return strings.Repeat(on, filled) + strings.Repeat(off, width-filled)
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
