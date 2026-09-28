# Proprietário e grupo

Cada inode possui UID e GID. Nomes são resolvidos pelas fontes NSS configuradas no host, portanto podem vir de arquivos locais, LDAP ou outras integrações do sistema.

- `chmod` não altera UID/GID.
- `chown usuario caminho` altera owner.
- `chown usuario:grupo caminho` altera ambos.
- `chgrp grupo caminho` ou `chown :grupo caminho` altera somente grupo.

## Operações no PermGuard

Na TUI, use `o` para alterar o proprietário e `G` para alterar somente o grupo. Na CLI:

```bash
sudo permguard --permitir-alteracoes alterar-owner /srv/app/uploads --usuario app
permguard --permitir-alteracoes alterar-grupo /srv/app/uploads --grupo web
```

As operações aceitam nome ou UID/GID, validam a identidade no sistema, mostram preview e exigem `ALTERAR`. `chown` exige root. Sem root, `chgrp` é permitido somente para item pertencente ao operador e para um de seus grupos efetivos. Symlinks, tipos especiais, caminhos críticos e filesystems somente leitura são bloqueados.

O estado atual mostra owner, grupo, UID e GID, mas altera apenas o modo. A futura alteração de owner/grupo exigirá existência da identidade, privilégio suficiente, preview, palavra `ALTERAR`, revalidação e auditoria.
