//go:build linux

package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

func TestInspecionaArquivoDiretorioELink(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "dados.txt")
	if err := os.WriteFile(file, []byte("dados fictícios"), 0o600); err != nil {
		t.Fatal(err)
	}
	// #nosec G302 -- modo é o objeto deste teste de inspeção.
	if err := os.Chmod(file, 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "atalho")
	if err := os.Symlink("dados.txt", link); err != nil {
		t.Fatal(err)
	}
	inspector := Inspector{}
	dirMeta, err := inspector.Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if dirMeta.Type != domain.TypeDirectory {
		t.Fatalf("tipo: %s", dirMeta.Type)
	}
	fileMeta, err := inspector.Inspect(file)
	if err != nil {
		t.Fatal(err)
	}
	if fileMeta.Type != domain.TypeRegular || fileMeta.Mode.NumericMode != "640" {
		t.Fatalf("arquivo: %+v", fileMeta)
	}
	linkMeta, err := inspector.Inspect(link)
	if err != nil {
		t.Fatal(err)
	}
	if !linkMeta.IsSymlink || linkMeta.Type != domain.TypeSymlink || linkMeta.SymlinkTarget != "dados.txt" {
		t.Fatalf("link: %+v", linkMeta)
	}
	if linkMeta.SymlinkTargetExists == nil || !*linkMeta.SymlinkTargetExists {
		t.Fatal("destino deveria existir")
	}
}

func TestLinkQuebradoECaminhoInexistente(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "quebrado")
	if err := os.Symlink("ausente", link); err != nil {
		t.Fatal(err)
	}
	meta, err := (Inspector{}).Inspect(link)
	if err != nil {
		t.Fatal(err)
	}
	if meta.SymlinkTargetExists == nil || *meta.SymlinkTargetExists {
		t.Fatal("destino deveria estar ausente")
	}
	if _, err = (Inspector{}).Inspect(filepath.Join(root, "nao-existe")); err == nil || err.Error() != "caminho_inexistente" {
		t.Fatalf("erro: %v", err)
	}
}

func TestReadDirNaoSegueLinkParaDiretorio(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "real")
	if err := os.Mkdir(child, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	entries, err := (Inspector{}).ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("itens: %d", len(entries))
	}
	for _, entry := range entries {
		if entry.Metadata.Name == "link" && entry.Metadata.Type != domain.TypeSymlink {
			t.Fatal("link foi seguido")
		}
	}
}
