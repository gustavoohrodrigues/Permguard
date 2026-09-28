# Histórico de mudanças

## Não publicado

- torna o navegador compacto e paginado, restaura a posição por diretório e aceita caminhos absolutos na pesquisa;
- redesenha o navegador com inspiração no ncdu, uso do filesystem, barras de tamanho e painel contextual;
- adiciona abertura confirmada de arquivos regulares no Vim com suspensão e restauração da TUI;
- restringe o editor a vim/nvim/vi, sem shell, sem symlinks e sem conteúdo na auditoria;
- adiciona alteração individual de proprietário (`chown`) e grupo proprietário (`chgrp`) na CLI e TUI;
- adiciona lookup de usuário/grupo por nome ou UID/GID, preview, confirmação e auditoria;
- revalida device, inode, modo, UID e GID antes de operações mutáveis de ownership;
- amplia a aba Explicações com disponibilidade e atalhos das operações de ownership;

### Adicionado

- Inspeção em modo somente leitura por padrão.
- CLI Cobra para inspeção e explicação de modos.
- TUI Bubble Tea com navegador, detalhes e explicação de permissões.
- Detecção de identidade, grupos, privilégios e capabilities relevantes.
- Inspeção segura por `Lstat`, sem seguir symlinks automaticamente.
- Chmod individual opt-in com preview, confirmação, revalidação e auditoria.
- Temas Midnight, Nord, Gruvbox Dark, Dracula e High Contrast.
- Alternância de cores e Unicode, filtros por tipo e ordenação.
- Menu de ajuda contextual e tela de auditoria.
