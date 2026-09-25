package permissions

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

var octalPattern = regexp.MustCompile(`^[0-7]{3,4}$`)

func Parse(value string) (os.FileMode, error) {
	if !octalPattern.MatchString(value) {
		return 0, fmt.Errorf("modo_octal_invalido")
	}
	n, err := strconv.ParseUint(value, 8, 32)
	if err != nil || n > 0o7777 {
		return 0, fmt.Errorf("modo_octal_invalido")
	}
	mode := os.FileMode(n & 0o777) // #nosec G115 -- n foi validado no intervalo octal 0000..7777.
	if n&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if n&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if n&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	return mode, nil
}

func FromFileMode(mode os.FileMode) domain.PermissionInfo {
	perm := uint32(mode.Perm())
	special := 0
	if mode&os.ModeSetuid != 0 {
		special |= 4
	}
	if mode&os.ModeSetgid != 0 {
		special |= 2
	}
	if mode&os.ModeSticky != 0 {
		special |= 1
	}
	numeric := fmt.Sprintf("%03o", perm)
	if special != 0 {
		numeric = fmt.Sprintf("%d%03o", special, perm)
	}
	return domain.PermissionInfo{
		NumericMode: numeric, SymbolicMode: symbolicMode(mode),
		OwnerRead: perm&0o400 != 0, OwnerWrite: perm&0o200 != 0, OwnerExecute: perm&0o100 != 0,
		GroupRead: perm&0o040 != 0, GroupWrite: perm&0o020 != 0, GroupExecute: perm&0o010 != 0,
		OthersRead: perm&0o004 != 0, OthersWrite: perm&0o002 != 0, OthersExecute: perm&0o001 != 0,
		SUID: mode&os.ModeSetuid != 0, SGID: mode&os.ModeSetgid != 0, Sticky: mode&os.ModeSticky != 0,
	}
}

func symbolicMode(mode os.FileMode) string {
	prefix := byte('-')
	switch {
	case mode.IsDir():
		prefix = 'd'
	case mode&os.ModeSymlink != 0:
		prefix = 'l'
	case mode&os.ModeSocket != 0:
		prefix = 's'
	case mode&os.ModeNamedPipe != 0:
		prefix = 'p'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		prefix = 'c'
	case mode&os.ModeDevice != 0:
		prefix = 'b'
	}
	bits := []byte("---------")
	flags := []struct {
		mask os.FileMode
		char byte
	}{
		{0o400, 'r'}, {0o200, 'w'}, {0o100, 'x'},
		{0o040, 'r'}, {0o020, 'w'}, {0o010, 'x'},
		{0o004, 'r'}, {0o002, 'w'}, {0o001, 'x'},
	}
	for i, flag := range flags {
		if mode&flag.mask != 0 {
			bits[i] = flag.char
		}
	}
	if mode&os.ModeSetuid != 0 {
		bits[2] = map[bool]byte{true: 's', false: 'S'}[bits[2] == 'x']
	}
	if mode&os.ModeSetgid != 0 {
		bits[5] = map[bool]byte{true: 's', false: 'S'}[bits[5] == 'x']
	}
	if mode&os.ModeSticky != 0 {
		bits[8] = map[bool]byte{true: 't', false: 'T'}[bits[8] == 'x']
	}
	return string(append([]byte{prefix}, bits...))
}

func Explain(info domain.PermissionInfo, directory bool) []domain.Explanation {
	return []domain.Explanation{
		explainScope("owner", info.OwnerRead, info.OwnerWrite, info.OwnerExecute, directory),
		explainScope("group", info.GroupRead, info.GroupWrite, info.GroupExecute, directory),
		explainScope("others", info.OthersRead, info.OthersWrite, info.OthersExecute, directory),
	}
}

func explainScope(scope string, read, write, execute, directory bool) domain.Explanation {
	bits := ""
	if read {
		bits += "r"
	} else {
		bits += "-"
	}
	if write {
		bits += "w"
	} else {
		bits += "-"
	}
	if execute {
		bits += "x"
	} else {
		bits += "-"
	}
	prefix := "file"
	if directory {
		prefix = "dir"
	}
	codes := []string{prefix + ".read.denied", prefix + ".write.denied", prefix + ".execute.denied"}
	if read {
		codes[0] = prefix + ".read.allowed"
	}
	if write {
		codes[1] = prefix + ".write.allowed"
	}
	if execute {
		codes[2] = prefix + ".execute.allowed"
	}
	return domain.Explanation{Scope: scope, Bits: bits, Codes: codes}
}
