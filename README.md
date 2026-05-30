# Feedback de Notas

Aplicacao Go + React para consulta mobile de notas por matricula em uma planilha do Google Sheets.

## Como funciona

- Login por matricula existente na aba `Base de dados`.
- Sessao em cookie HTTP-only assinado.
- Consulta somente leitura via Google Sheets API.
- Selecao entre `AB1` e `AB2`.
- Retorno apenas da linha da matricula logada.
- Comentarios por coluna sao lidos das notas das celulas do cabecalho da aba.

## Variaveis de ambiente

Crie as variaveis na Vercel e no ambiente local:

- `GOOGLE_SHEET_ID`: id da planilha.
- `LOGIN_SHEET_NAME`: aba com a base de matriculas.
- `SHEET_AB1_NAME`: aba de notas AB1.
- `SHEET_AB2_NAME`: aba de notas AB2.
- `SESSION_SECRET`: chave longa e aleatoria para assinar a sessao.
- `COOKIE_SECURE`: `true` em producao, `false` em dev local sem HTTPS.
- `GOOGLE_SERVICE_ACCOUNT_JSON`: JSON completo da service account.
- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`: alternativa ao JSON direto, boa para CI.
- `GOOGLE_SERVICE_ACCOUNT_FILE`: caminho local para o arquivo JSON da service account.

Compartilhe a planilha com o e-mail `client_email` da service account como leitor.

## Desenvolvimento

```bash
npm install
go mod download
npm run dev
```

A Vite dev server abre o frontend. Para testar as rotas Go localmente com a Vercel:

```bash
npx vercel dev
```

## Deploy

O projeto esta pronto para Vercel. Configure as variaveis acima no projeto e conecte o repositorio.

O workflow `.github/workflows/ci.yml` roda lint, build, testes Go e deploy com a Vercel CLI quando os secrets abaixo existirem:

- `VERCEL_TOKEN`
- `VERCEL_ORG_ID`
- `VERCEL_PROJECT_ID`
