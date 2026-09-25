# Arquitetura

O PermGuard separa interface, coleta e interpretação:

```text
Cobra CLI / Bubble Tea TUI
            │
       app + i18n
            │
 filesystem · permissions · identity · privilege
            │
   os.Lstat · os.ReadDir · os/user · statfs
```

`internal/filesystem` retorna modelos de domínio e códigos de erro, nunca texto de interface. `internal/permissions` converte modos e retorna códigos semânticos de explicação. A tradução ocorre em CLI/TUI. O catálogo pt-BR é incorporado ao binário.

O pacote `change` centraliza autorização, bloqueios, abertura segura, revalidação e `fchmod`. A TUI e a CLI não chamam syscalls mutáveis diretamente. O pacote `audit` grava somente metadados operacionais em JSONL restrito.

## Concorrência

Bubble Tea serializa atualizações do modelo. A leitura atual é limitada ao diretório aberto, sem varredura recursiva e sem carregar a árvore inteira.

## Portabilidade

Arquivos que dependem de `stat`, `statfs` e `/proc` usam build tag `linux`. Windows e macOS não fazem parte do MVP.
