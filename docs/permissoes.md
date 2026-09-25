# Permissões Unix

Um modo comum possui três dígitos: owner, group e others. Cada dígito soma leitura (`4`), escrita (`2`) e execução/acesso (`1`). Um quarto dígito inicial representa SUID (`4`), SGID (`2`) e sticky (`1`).

## Arquivos

- `r`: ler conteúdo.
- `w`: alterar conteúdo.
- `x`: executar quando formato e sistema permitirem.

## Diretórios

- `r`: listar nomes, sujeito às demais permissões.
- `w`: criar, renomear e remover entradas quando combinado com `x`.
- `x`: atravessar e acessar itens conhecidos.

## Bits especiais

- SUID em executável pode usar a identidade do proprietário e exige revisão cuidadosa.
- SGID em executável pode usar o grupo proprietário.
- SGID em diretório pode fazer novos itens herdarem o grupo.
- Sticky em diretório compartilhado normalmente limita remoção aos proprietários.

Use `permguard permissao explicar 770` para uma explicação em pt-BR.
