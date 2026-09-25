//go:build linux

package privilege

import (
	"bufio"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

func Detect() domain.PrivilegeInfo {
	info := domain.PrivilegeInfo{RealUID: syscall.Getuid(), EffectiveUID: syscall.Geteuid(), RealGID: syscall.Getgid(), EffectiveGID: syscall.Getegid()}
	if current, err := user.Current(); err == nil {
		info.User = current.Username
	}
	if info.User == "" {
		info.User = strconv.Itoa(info.EffectiveUID)
	}
	groupIDs, _ := syscall.Getgroups()
	groupIDs = append(groupIDs, info.EffectiveGID)
	seen := make(map[int]bool, len(groupIDs))
	for _, id := range groupIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		idText := strconv.Itoa(id)
		if group, err := user.LookupGroupId(idText); err == nil {
			info.Groups = append(info.Groups, group.Name)
		} else {
			info.Groups = append(info.Groups, idText)
		}
	}
	info.IsRoot = info.EffectiveUID == 0
	info.CapabilitiesHex = effectiveCapabilities()
	if value, err := strconv.ParseUint(info.CapabilitiesHex, 16, 64); err == nil {
		const relevant = (1 << 0) | (1 << 1) | (1 << 3)
		info.HasRelevantCapabilities = value&relevant != 0
	}
	info.Mode = classify(info.IsRoot, info.HasRelevantCapabilities)
	return info
}

func classify(isRoot, hasRelevantCapabilities bool) string {
	switch {
	case isRoot:
		return "administrative"
	case hasRelevantCapabilities:
		return "privileged"
	default:
		return "read_only"
	}
}

func effectiveCapabilities() string {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "CapEff:") {
			return strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "CapEff:"))
		}
	}
	return ""
}
