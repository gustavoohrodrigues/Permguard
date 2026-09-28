# Arquitetura

O PermGuard separa interface, coleta e interpretação:

```text
Cobra CLI / Bubble Tea TUI
            │
       app + i18n
            │
 filesystem · permissions · identity · privilege · editor
            │
 os.Lstat · os.ReadDir · os/user · statfs · exec direto
```

`internal/filesystem` retorna modelos de domínio e códigos de erro, nunca texto de interface. `internal/permissions` converte modos e retorna códigos semânticos de explicação. A tradução ocorre em CLI/TUI. O catálogo pt-BR é incorporado ao binário.

O pacote `change` centraliza autorização, bloqueios, abertura segura, revalidação, `fchmod` e `fchownat`. A TUI e a CLI não chamam syscalls mutáveis diretamente. O pacote `audit` grava somente metadados operacionais em JSONL restrito.

O navegador consulta somente o diretório atual. `statfs` fornece a capacidade agregada do filesystem, enquanto as barras por item representam apenas tamanhos conhecidos dos arquivos carregados; diretórios são identificados como `<DIR>` e não recebem um tamanho recursivo fictício.

O pacote `editor` restringe o comando externo a `vim`, `nvim` ou `vi`, aceita apenas arquivos regulares, bloqueia symlinks e executa o binário diretamente, sem shell. A TUI é suspensa pelo Bubble Tea enquanto o editor está ativo e restaurada ao término. O alvo é revalidado imediatamente antes da execução, embora um editor baseado em caminho não ofereça a mesma garantia contra TOCTOU de uma alteração feita por descritor aberto.

## Concorrência

Bubble Tea serializa atualizações do modelo. A leitura atual é limitada ao diretório aberto, sem varredura recursiva e sem carregar a árvore inteira.

## Portabilidade

Arquivos que dependem de `stat`, `statfs` e `/proc` usam build tag `linux`. Windows e macOS não fazem parte do MVP.
