# Segurança

## Princípios

1. Somente leitura por padrão; mutação exige `--permitir-alteracoes`.
2. `Lstat` antes de qualquer interpretação de item.
3. Symlinks não são seguidos automaticamente.
4. Nenhuma senha é solicitada ou processada.
5. Erros de permissão são visíveis, não ignorados silenciosamente.
6. Caminhos exibidos são absolutos após limpeza léxica.

## Privilégio

São exibidos UID/GID reais e efetivos, usuário, grupos, root e `CapEff`. Para chmod, o usuário efetivo deve ser root ou proprietário do item. Chown exige root. Chgrp exige root ou que o operador seja proprietário e membro do grupo proposto. A flag, o privilégio, o preview e a confirmação são verificações independentes.

## Mutação individual

O alvo é aberto com `O_NOFOLLOW`, revalidado por descritor e comparado por device, inode, modo, UID e GID. As alterações usam `fchmod` ou `fchownat` com `AT_EMPTY_PATH`, evitando trocar o alvo por pathname depois da validação. Symlinks, tipos especiais, mounts read-only, `/`, `/proc`, `/sys`, `/dev` e `/run` são bloqueados. Não existe recursão nem repetição automática.

A edição integrada aceita somente `vim`, `nvim` ou `vi`, resolvidos no `PATH`. O processo é iniciado com argumentos separados, sem shell, apenas para arquivo regular validado. A TUI exige `--permitir-alteracoes`, preview e confirmação `ALTERAR`; o conteúdo nunca é lido ou incluído na auditoria pelo PermGuard.

O alvo é revalidado por descritor imediatamente antes do lançamento do editor. Diferentemente de `fchmod` e `fchownat`, o editor externo precisa reabrir o caminho; portanto, não existe garantia anti-TOCTOU equivalente durante esse pequeno intervalo. Não use a edição integrada em diretórios controlados por usuários não confiáveis. Para mutações de permissões e ownership, permanece a garantia forte por descritor.

Outros caminhos sensíveis ainda receberão uma política de confirmação reforçada em incremento futuro.
