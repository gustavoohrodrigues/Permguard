//go:build linux

package change

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
)

type Service struct{ Inspector filesystem.Inspector }

func (s Service) Validate(metadata domain.FileMetadata, effectiveUID int) error {
	if err := s.validateTarget(metadata); err != nil {
		return err
	}
	ownerUID, err := strconv.Atoi(metadata.Owner.UID)
	if err != nil || (effectiveUID != 0 && effectiveUID != ownerUID) {
		return fmt.Errorf("privilegio_insuficiente")
	}
	return nil
}

func (s Service) ValidateOwner(metadata domain.FileMetadata, privileges domain.PrivilegeInfo) error {
	if err := s.validateTarget(metadata); err != nil {
		return err
	}
	if !privileges.IsRoot || privileges.EffectiveUID != 0 {
		return fmt.Errorf("chown_requer_root")
	}
	return nil
}

func (s Service) ValidateGroup(metadata domain.FileMetadata, groupID int, privileges domain.PrivilegeInfo) error {
	if err := s.validateTarget(metadata); err != nil {
		return err
	}
	if privileges.IsRoot && privileges.EffectiveUID == 0 {
		return nil
	}
	ownerUID, err := strconv.Atoi(metadata.Owner.UID)
	if err != nil || privileges.EffectiveUID != ownerUID {
		return fmt.Errorf("privilegio_grupo_insuficiente")
	}
	for _, effectiveGroupID := range privileges.GroupIDs {
		if effectiveGroupID == groupID {
			return nil
		}
	}
	return fmt.Errorf("grupo_nao_pertence_operador")
}

func (s Service) validateTarget(metadata domain.FileMetadata) error {
	if metadata.IsSymlink {
		return fmt.Errorf("symlink_mutacao_bloqueada")
	}
	if metadata.Type != domain.TypeRegular && metadata.Type != domain.TypeDirectory {
		return fmt.Errorf("tipo_mutacao_bloqueado")
	}
	if metadata.ReadOnlyFilesystem {
		return fmt.Errorf("filesystem_somente_leitura")
	}
	if blockedPath(metadata.Path) {
		return fmt.Errorf("caminho_critico_bloqueado")
	}
	return nil
}

func (s Service) ApplyMode(expected domain.FileMetadata, proposed os.FileMode, effectiveUID int) error {
	if err := s.Validate(expected, effectiveUID); err != nil {
		return err
	}
	fd, err := unix.Open(expected.Path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	pathOnly := false
	if err != nil {
		fd, err = unix.Open(expected.Path, unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		pathOnly = err == nil
	}
	if err != nil {
		return fmt.Errorf("falha_abertura_segura: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("falha_revalidacao: %w", err)
	}
	if err := revalidate(stat, expected); err != nil {
		return err
	}
	mode := uint32(proposed.Perm())
	if proposed&os.ModeSetuid != 0 {
		mode |= unix.S_ISUID
	}
	if proposed&os.ModeSetgid != 0 {
		mode |= unix.S_ISGID
	}
	if proposed&os.ModeSticky != 0 {
		mode |= unix.S_ISVTX
	}
	if pathOnly {
		err = unix.Chmod("/proc/self/fd/"+strconv.Itoa(fd), mode)
	} else {
		err = unix.Fchmod(fd, mode)
	}
	if err != nil {
		return fmt.Errorf("falha_chmod: %w", err)
	}
	return nil
}

func (s Service) ApplyOwnership(expected domain.FileMetadata, ownerID, groupID *int, privileges domain.PrivilegeInfo) error {
	if ownerID == nil && groupID == nil {
		return fmt.Errorf("alteracao_vazia")
	}
	if ownerID != nil {
		if err := s.ValidateOwner(expected, privileges); err != nil {
			return err
		}
	}
	if groupID != nil {
		if err := s.ValidateGroup(expected, *groupID, privileges); err != nil {
			return err
		}
	}
	fd, err := unix.Open(expected.Path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		fd, err = unix.Open(expected.Path, unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	}
	if err != nil {
		return fmt.Errorf("falha_abertura_segura: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("falha_revalidacao: %w", err)
	}
	if err := revalidate(stat, expected); err != nil {
		return err
	}
	uid, gid := -1, -1
	if ownerID != nil {
		uid = *ownerID
	}
	if groupID != nil {
		gid = *groupID
	}
	if err := unix.Fchownat(fd, "", uid, gid, unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("falha_chown: %w", err)
	}
	return nil
}

func revalidate(stat unix.Stat_t, expected domain.FileMetadata) error {
	ownerUID, ownerErr := strconv.Atoi(expected.Owner.UID)
	groupID, groupErr := strconv.Atoi(expected.Group.GID)
	if ownerErr != nil || groupErr != nil || stat.Ino != expected.Inode || uint64(stat.Dev) != expected.Filesystem.DeviceID || uint32(stat.Mode)&unix.S_IFMT == unix.S_IFLNK || int(stat.Uid) != ownerUID || int(stat.Gid) != groupID {
		return fmt.Errorf("alvo_alterado")
	}
	expectedMode, err := permissions.Parse(expected.Mode.NumericMode)
	if err != nil {
		return fmt.Errorf("falha_revalidacao: %w", err)
	}
	current := os.FileMode(stat.Mode & 0o777)
	if stat.Mode&unix.S_ISUID != 0 {
		current |= os.ModeSetuid
	}
	if stat.Mode&unix.S_ISGID != 0 {
		current |= os.ModeSetgid
	}
	if stat.Mode&unix.S_ISVTX != 0 {
		current |= os.ModeSticky
	}
	if current != expectedMode {
		return fmt.Errorf("alvo_alterado")
	}
	return nil
}

func blockedPath(path string) bool {
	clean := filepath.Clean(path)
	if clean == "/" {
		return true
	}
	for _, root := range []string{"/proc", "/sys", "/dev", "/run"} {
		if clean == root || len(clean) > len(root) && clean[:len(root)+1] == root+"/" {
			return true
		}
	}
	return false
}
