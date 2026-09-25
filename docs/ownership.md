# Proprietário e grupo

Cada inode possui UID e GID. Nomes são resolvidos pelas fontes NSS configuradas no host, portanto podem vir de arquivos locais, LDAP ou outras integrações do sistema.

- `chmod` não altera UID/GID.
- `chown usuario caminho` altera owner.
- `chown usuario:grupo caminho` altera ambos.
- `chgrp grupo caminho` ou `chown :grupo caminho` altera somente grupo.

O estado atual mostra owner, grupo, UID e GID, mas altera apenas o modo. A futura alteração de owner/grupo exigirá existência da identidade, privilégio suficiente, preview, palavra `ALTERAR`, revalidação e auditoria.
