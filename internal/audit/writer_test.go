package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

func TestAppendJSONLRestrito(t *testing.T) {
	path := filepath.Join(t.TempDir(), "estado", "audit.jsonl")
	record := domain.AuditRecord{Timestamp: time.Unix(1, 0), OperatorUser: "teste", TargetPath: "/caminho/ficticio", Operation: "chmod", PreviousMode: "600", NewMode: "640", Result: "sucesso"}
	if err := (Writer{Path: path}).Append(record); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("modo da auditoria: %o", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path) // #nosec G304 -- caminho temporário controlado pelo teste.
	text := string(data)
	if !strings.Contains(text, `"operation":"chmod"`) || strings.Contains(text, "conteudo") {
		t.Fatalf("registro inesperado: %s", text)
	}
}

func TestReadRecentRespeitaLimite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	writer := Writer{Path: path}
	for _, mode := range []string{"600", "640"} {
		if err := writer.Append(domain.AuditRecord{Timestamp: time.Now(), Operation: "chmod", NewMode: mode}); err != nil {
			t.Fatal(err)
		}
	}
	records, err := ReadRecent(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].NewMode != "640" {
		t.Fatalf("registros: %+v", records)
	}
}
