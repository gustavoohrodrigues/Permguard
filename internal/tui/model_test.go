package tui

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gustavoohrodrigues/permguard/internal/config"
	"github.com/gustavoohrodrigues/permguard/internal/domain"
	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/i18n"
)

type fakeChanger struct{ applied bool }

func (f *fakeChanger) Validate(domain.FileMetadata, int) error { return nil }
func (f *fakeChanger) ApplyMode(domain.FileMetadata, os.FileMode, int) error {
	f.applied = true
	return nil
}

type fakeAudit struct{ records int }

func (f *fakeAudit) Append(domain.AuditRecord) error { f.records++; return nil }

func testModel(t *testing.T) Model {
	t.Helper()
	c, err := i18n.Load("pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.StartPath = t.TempDir()
	return NewModel(Dependencies{Catalog: c, Inspector: filesystem.Inspector{}, Privilege: domain.PrivilegeInfo{User: "teste", EffectiveUID: 1000, Mode: "read_only"}, Config: cfg})
}
func TestTerminalEstreitoNaoEntraEmPanico(t *testing.T) {
	m := testModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	view := updated.(Model).View()
	if !strings.Contains(view, "PERMGUARD") {
		t.Fatal("cabeçalho ausente")
	}
}
func TestNavegacaoEntreTelas(t *testing.T) {
	m := testModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if updated.(Model).screen != 1 {
		t.Fatal("tela navegador não selecionada")
	}
}

func TestAbaExplicacoesIncluiOwnershipEExemplos(t *testing.T) {
	m := testModel(t)
	metadata := domain.FileMetadata{
		Path:  "/srv/app/uploads",
		Type:  domain.TypeDirectory,
		Mode:  domain.PermissionInfo{NumericMode: "2770", SymbolicMode: "rwxrws---", OwnerRead: true, OwnerWrite: true, OwnerExecute: true, GroupRead: true, GroupWrite: true, GroupExecute: true, SGID: true},
		Owner: domain.UserIdentity{Name: "app", UID: "1002"},
		Group: domain.GroupIdentity{Name: "web", GID: "1005"},
	}
	m.selected = &metadata
	m.width, m.height, m.screen = 120, 36, 3

	view := m.View()
	if !strings.Contains(view, "Tópico 1/3") || !strings.Contains(view, "SGID") {
		t.Fatalf("explicação atual inesperada: %s", view)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	view = m.View()
	if !strings.Contains(view, "OWNER, GROUP") || !strings.Contains(view, "chgrp") || !strings.Contains(view, "ainda não são") || !strings.Contains(view, "executados pelo PermGuard") {
		t.Fatalf("explicação de ownership ausente: %s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	view = m.View()
	for _, expected := range []string{"chmod 640", "chgrp web", "chown :web", "chown app:web", "chmod 2770"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("exemplo %q ausente: %s", expected, view)
		}
	}
}

func TestAlteracaoExigeConfirmacaoDigitada(t *testing.T) {
	m := testModel(t)
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := m.deps.Inspector.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	changer, auditor := &fakeChanger{}, &fakeAudit{}
	m.selected = &metadata
	m.deps.AllowWrites = true
	m.deps.Privilege.EffectiveUID = syscall.Geteuid()
	m.deps.Changer, m.deps.Audit = changer, auditor
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = updated.(Model)
	m.input.SetValue("640")
	updated, _ = m.submitInput()
	m = updated.(Model)
	if m.screen != 5 || m.pending == nil {
		t.Fatal("preview não foi criado")
	}
	m.inputMode = inputConfirmation
	m.input.SetValue("SIM")
	updated, cmd := m.submitInput()
	if cmd != nil || changer.applied {
		t.Fatal("confirmação inválida não pode aplicar")
	}
	m = updated.(Model)
	m.inputMode = inputConfirmation
	m.input.SetValue("ALTERAR")
	_, cmd = m.submitInput()
	if cmd == nil {
		t.Fatal("confirmação correta deveria gerar ação")
	}
	_ = cmd()
	if !changer.applied || auditor.records != 1 {
		t.Fatalf("aplicação=%v auditoria=%d", changer.applied, auditor.records)
	}
}
