package editor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

var allowed = map[string]bool{"vim": true, "nvim": true, "vi": true}

func Resolve(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "vim"
	}
	if strings.ContainsAny(command, " \t\r\n") || !allowed[filepath.Base(command)] {
		return "", fmt.Errorf("editor_nao_permitido")
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("editor_nao_encontrado")
	}
	return path, nil
}

func ValidateTarget(metadata domain.FileMetadata) error {
	if metadata.IsSymlink {
		return fmt.Errorf("symlink_edicao_bloqueada")
	}
	if metadata.Type != domain.TypeRegular {
		return fmt.Errorf("edicao_exige_arquivo_regular")
	}
	if metadata.ReadOnlyFilesystem {
		return fmt.Errorf("filesystem_somente_leitura")
	}
	return nil
}
