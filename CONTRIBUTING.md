# Contribuindo com o PermGuard

Toda interface, documentação, erro e exemplo devem permanecer em português brasileiro. Execute antes de enviar uma alteração:

```bash
make fmt
make vet
make race
make lint
make build
```

Testes que alteram metadados devem usar apenas `t.TempDir()`. Nunca teste contra caminhos reais como `/etc`, `/home`, `/var` ou `/tmp` global. Mudanças mutáveis exigem previamente preview, confirmação digitada, revalidação e auditoria.
