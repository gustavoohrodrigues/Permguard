package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPadraoSemArquivo(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "ausente.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "pt-BR" || !cfg.Security.AllowReadOnlyMode {
		t.Fatalf("config: %+v", cfg)
	}
}
func TestCarregaYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "language: pt-BR\nstart_path: /srv\ndisplay:\n  color_mode: sem_cor\n  show_help_bar: true\nsecurity:\n  follow_symlinks_for_read: true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StartPath != "/srv" || cfg.Display.ColorMode != "sem_cor" || !cfg.Display.ShowHelpBar || !cfg.Security.FollowSymlinksForRead {
		t.Fatalf("configuração: %+v", cfg)
	}
}
