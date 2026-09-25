# Roadmap

## Incrementos 1 e 2 — concluídos

- CLI/TUI, identidade, privilégio, navegação, pesquisa local, metadados, modos e symlinks.
- Preview e confirmação digitada para chmod individual.
- Revalidação device/inode/modo e auditoria JSONL.

## Próximo incremento — contexto avançado

- Detecção informativa de ACL, SELinux, capabilities, atributos e mount.
- Catálogo de usuários/grupos.
- Modelos de ChangeRequest/ChangePreview sem aplicação.
- Análise de caminhos sensíveis e permissões efetivas.

- `os.Chown` individual, nunca shell concatenado.

## Incremento 4 — relatórios e refinamento

- Exportação JSON/Markdown, redação de caminhos, filtros de auditoria e temas.

## Fora do MVP

Recursão completa, edição de ACL/SELinux/AppArmor/capabilities/atributos, SSH, múltiplos hosts, LDAP/AD administrativo, interface web, banco, exclusão, movimentação, cópia, criação de identidades, sudo automático e qualquer senha dentro do PermGuard.
