# Auditoria

A alteração individual de modo registra auditoria local depois da tentativa real.

O formato é JSONL com arquivo `0600`, usando `~/.local/state/permguard/audit.jsonl` para usuário ou `/var/log/permguard/audit.jsonl` para root. Registra operador, alvo, operação, modo anterior/proposto, resultado e indicador de symlink, nunca senha ou conteúdo de arquivo.

Visualização na TUI, filtros e exportações JSON/Markdown continuam planejados.
