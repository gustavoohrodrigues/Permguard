# Segurança

## Princípios

1. Somente leitura por padrão; mutação exige `--permitir-alteracoes`.
2. `Lstat` antes de qualquer interpretação de item.
3. Symlinks não são seguidos automaticamente.
4. Nenhuma senha é solicitada ou processada.
5. Erros de permissão são visíveis, não ignorados silenciosamente.
6. Caminhos exibidos são absolutos após limpeza léxica.

## Privilégio

São exibidos UID/GID reais e efetivos, usuário, grupos, root e `CapEff`. Para chmod, o usuário efetivo deve ser root ou proprietário do item. A flag, o privilégio, o preview e a confirmação são verificações independentes.

## Mutação individual

O alvo é aberto com `O_NOFOLLOW`, revalidado por descritor e comparado por device, inode e modo. A alteração usa `fchmod`, evitando trocar o alvo por pathname depois da validação. Symlinks, tipos especiais, mounts read-only, `/`, `/proc`, `/sys`, `/dev` e `/run` são bloqueados. Não existe recursão nem repetição automática.

Outros caminhos sensíveis ainda receberão uma política de confirmação reforçada em incremento futuro.
