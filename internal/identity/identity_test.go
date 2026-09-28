package identity

import (
	"os/user"
	"testing"
)

func TestLookupUserEGroupAtuais(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skipf("usuário atual indisponível: %v", err)
	}
	identity, uid, err := LookupUser(current.Uid)
	if err != nil || identity.UID != current.Uid || uid < 0 {
		t.Fatalf("usuário inesperado: %+v %d %v", identity, uid, err)
	}
	group, gid, err := LookupGroup(current.Gid)
	if err != nil || group.GID != current.Gid || gid < 0 {
		t.Fatalf("grupo inesperado: %+v %d %v", group, gid, err)
	}
}

func TestLookupRejeitaEntradasInvalidas(t *testing.T) {
	if _, _, err := LookupUser("root:root"); err == nil {
		t.Fatal("usuário inválido deveria falhar")
	}
	if _, _, err := LookupGroup("grupo/invalido"); err == nil {
		t.Fatal("grupo inválido deveria falhar")
	}
}
