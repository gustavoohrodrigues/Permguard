//go:build linux

package change

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/gustavoohrodrigues/permguard/internal/domain"
	"github.com/gustavoohrodrigues/permguard/internal/filesystem"
	"github.com/gustavoohrodrigues/permguard/internal/permissions"
)

func TestApplyModeComRevalidacao(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, []byte("fictício"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, err := inspector.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	mode, _ := permissions.Parse("640")
	service := Service{Inspector: inspector}
	if err := service.ApplyMode(metadata, mode, syscall.Geteuid()); err != nil {
		t.Fatal(err)
	}
	updated, _ := inspector.Inspect(path)
	if updated.Mode.NumericMode != "640" {
		t.Fatalf("modo final: %s", updated.Mode.NumericMode)
	}
}

func TestApplyOwnershipGrupoAtualComRevalidacao(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, err := inspector.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	gid := syscall.Getegid()
	privileges := domain.PrivilegeInfo{EffectiveUID: syscall.Geteuid(), EffectiveGID: gid, IsRoot: syscall.Geteuid() == 0, GroupIDs: append([]int{gid}, mustGroups(t)...)}
	if err := (Service{Inspector: inspector}).ApplyOwnership(metadata, nil, &gid, privileges); err != nil {
		t.Fatal(err)
	}
}

func TestApplyOwnershipOwnerAtualQuandoRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("chown exige root")
	}
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, err := inspector.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	uid := syscall.Geteuid()
	privileges := domain.PrivilegeInfo{EffectiveUID: uid, IsRoot: true, GroupIDs: []int{syscall.Getegid()}}
	if err := (Service{Inspector: inspector}).ApplyOwnership(metadata, &uid, nil, privileges); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOwnerExigeRoot(t *testing.T) {
	metadata := domain.FileMetadata{Type: domain.TypeRegular, Owner: domain.UserIdentity{UID: "1000"}}
	err := (Service{}).ValidateOwner(metadata, domain.PrivilegeInfo{EffectiveUID: 1000})
	if err == nil || !strings.Contains(err.Error(), "chown_requer_root") {
		t.Fatalf("chown deveria exigir root: %v", err)
	}
}

func TestValidateGroupExigeMembroDoGrupo(t *testing.T) {
	metadata := domain.FileMetadata{Type: domain.TypeRegular, Owner: domain.UserIdentity{UID: "1000"}}
	service := Service{}
	if err := service.ValidateGroup(metadata, 2000, domain.PrivilegeInfo{EffectiveUID: 1000, GroupIDs: []int{1000}}); err == nil || !strings.Contains(err.Error(), "grupo_nao_pertence") {
		t.Fatalf("grupo externo deveria ser bloqueado: %v", err)
	}
	if err := service.ValidateGroup(metadata, 1000, domain.PrivilegeInfo{EffectiveUID: 1000, GroupIDs: []int{1000}}); err != nil {
		t.Fatalf("grupo efetivo deveria ser aceito: %v", err)
	}
}

func TestApplyOwnershipCancelaQuandoOwnerEsperadoMuda(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, _ := inspector.Inspect(path)
	metadata.Owner.UID = "999999"
	gid := syscall.Getegid()
	privileges := domain.PrivilegeInfo{EffectiveUID: 0, IsRoot: true, GroupIDs: []int{gid}}
	err := (Service{Inspector: inspector}).ApplyOwnership(metadata, nil, &gid, privileges)
	if err == nil || !strings.Contains(err.Error(), "alvo_alterado") {
		t.Fatalf("mudança de owner deveria cancelar: %v", err)
	}
}

func mustGroups(t *testing.T) []int {
	t.Helper()
	groups, err := syscall.Getgroups()
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

func TestCancelaQuandoInodeMuda(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(path, []byte("fictício"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := filesystem.Inspector{}
	metadata, _ := inspector.Inspect(path)
	metadata.Inode++
	mode, _ := permissions.Parse("640")
	err := (Service{Inspector: inspector}).ApplyMode(metadata, mode, syscall.Geteuid())
	if err == nil || !strings.Contains(err.Error(), "alvo_alterado") {
		t.Fatalf("erro esperado de revalidação; recebido: %v", err)
	}
}

func TestBloqueiaSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "alvo")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	metadata, _ := (filesystem.Inspector{}).Inspect(link)
	if err := (Service{}).Validate(metadata, syscall.Geteuid()); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink deveria ser bloqueado: %v", err)
	}
}
