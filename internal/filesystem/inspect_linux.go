//go:build linux

package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
	"github.com/gustavoohrodrigues/permguard/internal/identity"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
)

type Inspector struct{}

func (Inspector) Inspect(path string) (domain.FileMetadata, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return domain.FileMetadata{}, fmt.Errorf("caminho_invalido")
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.FileMetadata{}, fmt.Errorf("caminho_inexistente")
		}
		if errors.Is(err, os.ErrPermission) {
			return domain.FileMetadata{}, fmt.Errorf("permissao_negada")
		}
		return domain.FileMetadata{}, fmt.Errorf("falha_inspecao")
	}
	metadata := domain.FileMetadata{Path: absolute, ResolvedPath: absolute, Name: info.Name(), Type: fileType(info.Mode()), Size: info.Size(), Mode: permissions.FromFileMode(info.Mode()), ModifiedAt: info.ModTime(), IsSymlink: info.Mode()&os.ModeSymlink != 0}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid, gid := strconv.FormatUint(uint64(stat.Uid), 10), strconv.FormatUint(uint64(stat.Gid), 10)
		metadata.Owner = domain.UserIdentity{Name: identity.UserByID(uid), UID: uid}
		metadata.Group = domain.GroupIdentity{Name: identity.GroupByID(gid), GID: gid}
		metadata.Inode = stat.Ino
		atime, ctime := time.Unix(stat.Atim.Sec, stat.Atim.Nsec), time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
		metadata.AccessedAt, metadata.ChangedAt = &atime, &ctime
		metadata.Filesystem.DeviceID = uint64(stat.Dev)
	}
	var fs syscall.Statfs_t
	if syscall.Statfs(absolute, &fs) == nil {
		metadata.ReadOnlyFilesystem = fs.Flags&unix.ST_RDONLY != 0
		metadata.Filesystem.ReadOnly = metadata.ReadOnlyFilesystem
	}
	if metadata.IsSymlink {
		target, readErr := os.Readlink(absolute)
		if readErr == nil {
			metadata.SymlinkTarget = target
			resolved := target
			if !filepath.IsAbs(target) {
				resolved = filepath.Join(filepath.Dir(absolute), target)
			}
			_, targetErr := os.Stat(resolved)
			exists := targetErr == nil
			metadata.SymlinkTargetExists = &exists
		}
	}
	return metadata, nil
}

func (i Inspector) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	parent, err := i.Inspect(path)
	if err != nil {
		return nil, err
	}
	if parent.Type != domain.TypeDirectory {
		return nil, fmt.Errorf("nao_e_diretorio")
	}
	entries, err := os.ReadDir(parent.Path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("permissao_negada")
		}
		return nil, fmt.Errorf("falha_listagem")
	}
	result := make([]domain.DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		metadata, inspectErr := i.Inspect(filepath.Join(parent.Path, entry.Name()))
		item := domain.DirectoryEntry{Metadata: metadata}
		if inspectErr != nil {
			item.Metadata.Path = filepath.Join(parent.Path, entry.Name())
			item.Metadata.Name = entry.Name()
			item.ErrorCode = inspectErr.Error()
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(a, b int) bool {
		ad, bd := result[a].Metadata.Type == domain.TypeDirectory, result[b].Metadata.Type == domain.TypeDirectory
		if ad != bd {
			return ad
		}
		return result[a].Metadata.Name < result[b].Metadata.Name
	})
	return result, nil
}

func fileType(mode os.FileMode) domain.FileType {
	switch {
	case mode&os.ModeSymlink != 0:
		return domain.TypeSymlink
	case mode.IsDir():
		return domain.TypeDirectory
	case mode.IsRegular():
		return domain.TypeRegular
	case mode&os.ModeSocket != 0:
		return domain.TypeSocket
	case mode&os.ModeNamedPipe != 0:
		return domain.TypeFIFO
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return domain.TypeCharacter
	case mode&os.ModeDevice != 0:
		return domain.TypeBlock
	default:
		return domain.TypeUnknown
	}
}
