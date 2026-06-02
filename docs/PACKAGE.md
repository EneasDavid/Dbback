# GitHub Package

O pacote npm do projeto esta configurado como `@eneasdavid/dbback` e usa o registry do GitHub Packages.

## Gerar localmente

```bash
npm run package:dry-run
npm run package
```

## Publicar

Publique uma release no GitHub ou rode manualmente o workflow `publish-package`.

Antes de republicar, incremente `version` em `package.json`; o GitHub Packages nao aceita sobrescrever a mesma versao.
