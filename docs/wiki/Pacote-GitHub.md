# Pacote GitHub

O pacote npm do projeto fica configurado como:

```text
@eneasdavid/dbback
```

Ele e publicado no GitHub Packages pelo workflow `publish-package`.

## Criar pacote local

```bash
npm run package:dry-run
npm run package
```

O `prepack` executa `npm run build` antes de gerar o tarball.

## Publicar no GitHub Packages

1. Atualize `version` em `package.json`.
2. Garanta que `npm run lint` e `npm run build` passam.
3. Publique uma release no GitHub ou execute o workflow `publish-package` manualmente.

O workflow usa `GITHUB_TOKEN`, entao nao coloque token pessoal no repositorio.

## Instalar o pacote

Configure o registry no projeto consumidor:

```text
@eneasdavid:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${GITHUB_TOKEN}
```

Depois instale:

```bash
npm install @eneasdavid/dbback
```

Se a versao ja existir no GitHub Packages, incremente `version` antes de publicar de novo.
