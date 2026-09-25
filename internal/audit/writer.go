package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

type Writer struct{ Path string }

func ReadRecent(path string, limit int) ([]domain.AuditRecord, error) {
	if limit < 1 {
		limit = 1
	}
	file, err := os.Open(path) // #nosec G304 -- caminho de auditoria configurado pelo operador.
	if errors.Is(err, os.ErrNotExist) {
		return []domain.AuditRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("falha_leitura_auditoria: %w", err)
	}
	defer func() { _ = file.Close() }()
	records := make([]domain.AuditRecord, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var record domain.AuditRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue
		}
		if len(records) == limit {
			copy(records, records[1:])
			records[len(records)-1] = record
		} else {
			records = append(records, record)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("falha_leitura_auditoria: %w", err)
	}
	return records, nil
}

func Path(userPath, rootPath string, root bool) (string, error) {
	value := userPath
	if root {
		value = rootPath
	}
	if strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	return filepath.Abs(value)
}

func (w Writer) Append(record domain.AuditRecord) error {
	if err := os.MkdirAll(filepath.Dir(w.Path), 0o700); err != nil {
		return fmt.Errorf("falha_auditoria: %w", err)
	}
	file, err := os.OpenFile(w.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- destino configurado pelo operador.
	if err != nil {
		return fmt.Errorf("falha_auditoria: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("falha_auditoria: %w", err)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("falha_auditoria: %w", err)
	}
	return file.Sync()
}
