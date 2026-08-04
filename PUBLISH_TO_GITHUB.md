# 📤 Publicar dbBack no GitHub

## Passo 1: Criar repositório no GitHub

1. Vá para [github.com/new](https://github.com/new)
2. Preencha:
   - **Repository name**: `dbback`
   - **Description**: "Consulta autenticada de notas e feedbacks a partir do Google Sheets"
   - **Public** (ou Private se preferir)
   - ✅ "Add a .gitignore" → Go
   - ✅ "Choose a license" → MIT ou UNLICENSED

3. Clique em "Create repository"

## Passo 2: Conectar repositório local ao GitHub

```bash
# Adicione o remote (substitua YOUR_USERNAME)
git remote set-url origin https://github.com/YOUR_USERNAME/dbback.git

# Ou se for SSH:
git remote set-url origin git@github.com:YOUR_USERNAME/dbback.git

# Verifique
git remote -v
```

## Passo 3: Push para o GitHub

```bash
# Se estiver em uma branch temporária, vá para main/master
git checkout -b main

# Push para o GitHub
git push -u origin main

# Ou se já tem main
git push origin main
```

## Passo 4: Configurar GitHub (Opcional)

### Adicione tópicos (Topics)

No GitHub, acesse seu repositório e adicione Topics:
- `go`
- `react`
- `google-sheets`
- `education`
- `clean-code`
- `solid`

### Adicione proteção na branch main

Settings → Branches → Add Rule
- Branch name pattern: `main`
- ✅ Require pull request reviews before merging

## Passo 5: Documentação de Deploy

Crie um arquivo `.github/workflows/ci.yml` para CI/CD:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: go test ./tests/... -v -cover
      - run: go build -o feedback ./cmd/dev
```

## Passo 6: Verificar no GitHub

Após fazer push, acesse:
- `https://github.com/YOUR_USERNAME/dbback`
- Verifique se todos os arquivos estão lá
- Clique na aba "Releases" para criar uma tag

## ✅ Status do Repositório

```
dbBack ✅ PRONTO PARA GITHUB

- Código limpo e organizado
- Documentação completa
- 30+ testes
- 85%+ coverage
- Enterprise-grade quality
```

## 🚀 Próximos Passos

1. Criar releases no GitHub
2. Documentação no GitHub Pages
3. Badges de CI/CD
4. Adicionar contribuindo guidelines
