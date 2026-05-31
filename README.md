# dbBack

Monolito Go + React para consulta autenticada de notas e feedbacks a partir de Google Sheets. A aplicacao valida a matricula na aba de login, resolve a identidade do aluno, le somente a linha correspondente nas abas avaliativas e entrega um payload pronto para a UI.

## Documentacao da API

A documentacao de rotas, schemas e organizacao do payload fica na propria API:

- Local: `GET /api/docs`
- Vercel: <https://dbback-nxak8qw9b-eneasdavids-projects.vercel.app/api/docs>

A rota de docs usa Basic Auth. O usuario e a senha existem somente no ambiente da aplicacao; nao registre valores de login no README.

## Arquitetura

O projeto segue uma organizacao MVC pragmatica para Go:

- `api/`: roteador serverless, controllers HTTP e view HTML das docs.
- `pkg/app/`: modelos, configuracao, sessoes, servicos de notas e repositorio Google Sheets/Drive.
- `cmd/dev/`: servidor local que serve frontend e API no mesmo processo.
- `cmd/comments/`: utilitario de diagnostico para comentarios vistos pela service account.
- `src/`: UI React, cliente HTTP, tipos e componentes.

Fluxo principal:

```text
React/Vite -> api Router -> Controllers -> pkg/app services -> Google Sheets/Drive
```

Os comentarios ricos do Google Drive sao usados apenas como enriquecimento. Se a Drive API ou os comentarios falharem, a leitura dos valores da planilha via Sheets continua funcionando.

## Configuracao

Copie `env.example` para `.env` no desenvolvimento local e configure as variaveis do projeto. As principais sao:

```env
GOOGLE_SHEET_ID=...
LOGIN_SHEET_NAME=Base de dados
SHEET_AB1_PESQUISA=AT. 1
SHEET_AB1_ARTIGO=AT. 2
SHEET_AB1_LISTA=AT. 3
SHEET_AB1_PROVA=Notas AB1
SHEET_AB2_LISTA=AT. 4
SHEET_AB2_PROJETO=Projeto AB2
SESSION_SECRET=<chave-aleatoria-com-32-bytes-ou-mais>
COOKIE_SECURE=true
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<credencial-json-em-base64>
```

Tambem configure no ambiente as credenciais de Basic Auth da documentacao da API. O projeto nao aceita credenciais padrao para essa rota.

Credenciais Google aceitas:

- `GOOGLE_SERVICE_ACCOUNT_JSON`
- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`
- `GOOGLE_SERVICE_ACCOUNT_FILE` apenas em desenvolvimento local

Compartilhe a planilha com o `client_email` da service account. Para comentarios ricos, habilite a Drive API no projeto Google Cloud; sem ela, os valores da planilha ainda sao lidos.

## Desenvolvimento

```bash
npm install
go mod download
npm run dev:full
```

O servidor local usa `PORT` e, quando a variavel nao e definida, sobe na porta `3000`.

Comandos de qualidade:

```bash
go test ./...
npm run lint
npm run build
```

Diagnostico de comentarios:

```bash
go run ./cmd/comments
go run ./cmd/comments -matricula 2024001339 -exam ab1
go run ./cmd/comments -raw-drive
GOOGLE_SERVICE_ACCOUNT_FILE=./service-account.local.json PORT=3000 bash test-comments.sh
```

## Deploy

No Vercel, nao use `GOOGLE_SERVICE_ACCOUNT_FILE`. Gere a credencial em base64:

```bash
base64 < service-account.local.json | tr -d '\n'
```

Configure no provedor:

```env
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<saida>
GOOGLE_SHEET_ID=<id>
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
```

Mantenha `VITE_API_BASE` vazio quando frontend e `/api/*` estiverem no mesmo projeto.
