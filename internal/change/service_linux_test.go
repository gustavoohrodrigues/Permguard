//go:build linux

package change

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
)

func TestApplyModeComRevalidacao(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, []byte("fictício"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, err := inspector.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	mode, _ := permissions.Parse("640")
	service := Service{Inspector: inspector}
	if err := service.ApplyMode(metadata, mode, syscall.Geteuid()); err != nil {
		t.Fatal(err)
	}
	updated, _ := inspector.Inspect(path)
	if updated.Mode.NumericMode != "640" {
		t.Fatalf("modo final: %s", updated.Mode.NumericMode)
	}
}

func TestCancelaQuandoInodeMuda(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, []byte("fictício"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, _ := inspector.Inspect(path)
	metadata.Inode++
	mode, _ := permissions.Parse("640")
	err := (Service{Inspector: inspector}).ApplyMode(metadata, mode, syscall.Geteuid())
	if err == nil || !strings.Contains(err.Error(), "alvo_alterado") {
		t.Fatalf("erro esperado de revalidação; recebido: %v", err)
	}
}

func TestBloqueiaSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "alvo")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	metadata, _ := (filesystem.Inspector{}).Inspect(link)
	if err := (Service{}).Validate(metadata, syscall.Geteuid()); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink deveria ser bloqueado: %v", err)
	}
}
