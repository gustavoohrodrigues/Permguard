package identity

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
)

func UserByID(id string) (name string) {
	u, err := user.LookupId(id)
	if err != nil {
		return id
	}
	return u.Username
}

func GroupByID(id string) (name string) {
	g, err := user.LookupGroupId(id)
	if err != nil {
		return id
	}
	return g.Name
}

func LookupUser(value string) (domain.UserIdentity, int, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, ":/\\\x00\n\r") {
		return domain.UserIdentity{}, 0, fmt.Errorf("usuario_invalido")
	}
	entry, err := user.Lookup(value)
	if err != nil {
		entry, err = user.LookupId(value)
	}
	if err != nil {
		return domain.UserIdentity{}, 0, fmt.Errorf("usuario_inexistente")
	}
	id, err := strconv.Atoi(entry.Uid)
	if err != nil || id < 0 {
		return domain.UserIdentity{}, 0, fmt.Errorf("usuario_invalido")
	}
	return domain.UserIdentity{Name: entry.Username, UID: entry.Uid}, id, nil
}

func LookupGroup(value string) (domain.GroupIdentity, int, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, ":/\\\x00\n\r") {
		return domain.GroupIdentity{}, 0, fmt.Errorf("grupo_invalido")
	}
	entry, err := user.LookupGroup(value)
	if err != nil {
		entry, err = user.LookupGroupId(value)
	}
	if err != nil {
		return domain.GroupIdentity{}, 0, fmt.Errorf("grupo_inexistente")
	}
	id, err := strconv.Atoi(entry.Gid)
	if err != nil || id < 0 {
		return domain.GroupIdentity{}, 0, fmt.Errorf("grupo_invalido")
	}
	return domain.GroupIdentity{Name: entry.Name, GID: entry.Gid}, id, nil
}
