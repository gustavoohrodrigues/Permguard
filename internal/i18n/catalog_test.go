package i18n

import (
	"strings"
	"testing"
)

func TestCatalogoPortuguesCompletoParaChavesCentrais(t *testing.T) {
	c, err := Load("pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"app.name", "screen.browser", "error.permissao_negada", "footer.keys"} {
		if value := c.T(key); strings.HasPrefix(value, "[") {
			t.Errorf("chave ausente: %s", key)
		}
	}
}
func TestIdiomaNaoSuportado(t *testing.T) {
	if _, err := Load("xx"); err == nil {
		t.Fatal("deveria rejeitar idioma")
	}
}
