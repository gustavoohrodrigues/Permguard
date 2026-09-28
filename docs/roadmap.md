# Roadmap

## Incrementos 1 e 2 — concluídos

- CLI/TUI, identidade, privilégio, navegação, pesquisa local, metadados, modos e symlinks.
- Preview e confirmação digitada para chmod, chown e chgrp individuais. **Concluído.**
- Revalidação device/inode/modo e auditoria JSONL.
- Navegador inspirado no ncdu e edição confirmada com Vim. **Concluído.**

## Próximo incremento — contexto avançado

- Detecção informativa de ACL, SELinux, capabilities, atributos e mount.
- Catálogo de usuários/grupos.
- Cálculo assíncrono e cancelável do tamanho recursivo de diretórios, sem seguir symlinks.
- Modelos de ChangeRequest/ChangePreview sem aplicação.
- Análise de caminhos sensíveis e permissões efetivas.

## Incremento 4 — relatórios e refinamento

- Exportação JSON/Markdown, redação de caminhos, filtros de auditoria e temas.

## Fora do MVP

Recursão completa, edição de ACL/SELinux/AppArmor/capabilities/atributos, SSH, múltiplos hosts, LDAP/AD administrativo, interface web, banco, exclusão, movimentação, cópia, criação de identidades, sudo automático e qualquer senha dentro do PermGuard.
