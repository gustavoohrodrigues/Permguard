package editor

import (
	"strings"
	"testing"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

func TestResolveBloqueiaComandoComArgumentosOuForaDaLista(t *testing.T) {
	for _, value := range []string{"vim -c quit", "sh", "editor;id"} {
		if _, err := Resolve(value); err == nil || !strings.Contains(err.Error(), "editor_nao_permitido") {
			t.Fatalf("editor %q deveria ser bloqueado: %v", value, err)
		}
	}
}

func TestValidateTargetBloqueiaDiretorioESymlink(t *testing.T) {
	for _, metadata := range []domain.FileMetadata{{Type: domain.TypeDirectory}, {Type: domain.TypeSymlink, IsSymlink: true}} {
		if err := ValidateTarget(metadata); err == nil {
			t.Fatalf("alvo deveria ser bloqueado: %+v", metadata)
		}
	}
}

func TestValidateTargetAceitaArquivoRegular(t *testing.T) {
	if err := ValidateTarget(domain.FileMetadata{Type: domain.TypeRegular}); err != nil {
		t.Fatal(err)
	}
}
