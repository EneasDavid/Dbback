# Instalacao

## Requisitos

- Node.js 22
- npm
- Go 1.25.x
- Acesso a uma planilha Google Sheets compartilhada com a service account

## Preparar o ambiente

```bash
npm install
go mod download
cp env.example .env
```

Edite `.env` com os IDs das planilhas, credenciais Google e segredo de sessao.

## Rodar localmente

Frontend e API no mesmo processo:

```bash
npm run dev:full
```

Somente frontend Vite:

```bash
npm run dev
```

Use o modo completo quando precisar testar login, sessao e rotas `/api/*`.

## Qualidade

```bash
go test ./...
npm run lint
npm run build
```

O workflow `quality-gate` executa checks de higiene, backend e frontend em pull requests e pushes para `main`.
