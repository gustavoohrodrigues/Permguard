package tui

import "testing"

func TestTodosOsTemasRenderizam(t *testing.T) {
	for _, name := range themeOrder {
		for _, colors := range []bool{true, false} {
			if output := theme(name, colors, true).title.Render("PermGuard"); output == "" {
				t.Fatalf("tema %s não renderizou", name)
			}
		}
	}
}

func TestCicloDeTemas(t *testing.T) {
	current := "midnight"
	for range themeOrder {
		current = nextTheme(current)
	}
	if current != "midnight" {
		t.Fatalf("ciclo terminou em %s", current)
	}
}
