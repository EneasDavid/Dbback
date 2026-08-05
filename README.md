# dbBack

Aplicação web para consulta autenticada de notas e feedbacks armazenados no Google Sheets. O backend em Go identifica o aluno, lê somente os dados associados à matrícula autenticada e entrega um payload pronto para o frontend React.

## Funcionalidades

- Login por matrícula numérica com sessão assinada em cookie `HttpOnly`.
- Login opcional por passkey depois do primeiro acesso.
- Cloudflare Turnstile opcional no formulário de login.
- Suporte a uma ou várias planilhas, mantendo a sessão vinculada à origem correta.
- Avaliações e atividades descobertas pelas abas de controle da planilha.
- Feedback obtido de notas de célula, comentários do arquivo XLSX e Google Drive quando disponíveis.
- Cache em memória no backend e cache SWR em `sessionStorage` no frontend.
- Respostas de notas com `ETag` e cache HTTP privado.
- API e interface servidas pelo mesmo projeto no Vercel ou pelo servidor Go local.

## Tecnologias

- Go 1.26.5
- React 19 e TypeScript
- Vite 8
- Google Sheets API e Google Drive API
- Vercel

## Estrutura

```text
api/                entrada serverless da Vercel
cmd/dev/            servidor local do frontend e da API
cmd/comments/       diagnóstico dos comentários visíveis à service account
pkg/app/            configuração, Google Sheets/Drive, sessão e regras de notas
pkg/httpapi/        rotas e controllers HTTP
src/Controllers/    fluxo da interface e chamadas da API
src/Models/         tipos, normalização e cache do frontend
src/Views/          componentes React e estilos
docs/wiki/          conteúdo sincronizado com a GitHub Wiki
```

## Estrutura esperada no Google Sheets

A aba definida por `LOGIN_SHEET_NAME` é usada no login e precisa ter colunas reconhecíveis de matrícula e nome.

O modelo de notas usa:

- `abs`: avaliações disponíveis; somente linhas ativas (`status = 1`) aparecem na aplicação.
- `atividades`: atividades, pesos, avaliação correspondente e nome da aba de detalhes.
- `nota <avaliação>`: resumo das notas do aluno, por exemplo `nota ab1`.
- Abas das atividades: critérios, grupos, notas e feedbacks detalhados.

Os nomes das avaliações são descobertos na aba `abs`; não estão limitados a AB1 e AB2. O nome de cada aba de atividade vem da aba `atividades`.

## Configuração

Copie o arquivo de exemplo:

```bash
cp env.example .env
```

Variáveis principais:

```env
GOOGLE_SHEET_V2_IDS=id_da_planilha_1,id_da_planilha_2
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=uma-chave-aleatoria-forte
COOKIE_SECURE=false
GOOGLE_SERVICE_ACCOUNT_FILE=./service-account.local.json
```

`GOOGLE_SHEET_V2_IDS` aceita IDs separados por vírgula, ponto e vírgula ou quebra de linha. O backend também reconhece `GOOGLE_SHEET_ID` e `GOOGLE_SHEET_IDS` por compatibilidade; IDs repetidos são descartados. A ordem efetiva é `GOOGLE_SHEET_V2_IDS`, `GOOGLE_SHEET_ID` e `GOOGLE_SHEET_IDS`.

Credenciais Google aceitas:

- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`: recomendada em produção.
- `GOOGLE_SERVICE_ACCOUNT_JSON`: JSON completo em uma variável.
- `GOOGLE_SERVICE_ACCOUNT_FILE`: arquivo local para desenvolvimento.

A planilha precisa ser compartilhada como leitora com o `client_email` da service account. Habilite a Google Sheets API; habilite também a Google Drive API para enriquecer o payload com comentários do Drive.

### Variáveis opcionais

```env
TURNSTILE_SECRET_KEY=
VITE_TURNSTILE_SITE_KEY=
DOCS_USERNAME=
DOCS_PASSWORD=
VITE_API_BASE=
PORT=3000
```

- Turnstile só é ativado quando as chaves pública e privada são configuradas.
- `/api/docs` só fica disponível quando usuário e senha de documentação estão configurados.
- `COOKIE_SECURE` deve ser `true` em produção e `false` no desenvolvimento HTTP local.
- `VITE_API_BASE` normalmente fica vazio quando frontend e API usam o mesmo domínio.

Nunca versione `.env`, arquivos de service account ou segredos.

## Desenvolvimento local

Requisitos: Node.js 22, npm e Go 1.26.5.

```bash
npm ci
go mod download
npm run dev:full
```

A aplicação completa fica disponível em `http://localhost:3000`. O comando gera o frontend e inicia o servidor Go no mesmo host.

Para trabalhar apenas no frontend:

```bash
npm run dev
```

Nesse modo, o Vite encaminha `/api` para `http://localhost:3000`; mantenha o backend em execução separadamente se precisar das rotas reais.

## API

| Método | Rota | Descrição |
| --- | --- | --- |
| `POST` | `/api/login` | Valida matrícula e cria a sessão. |
| `POST` | `/api/logout` | Encerra a sessão. |
| `GET` | `/api/me` | Retorna o usuário atual ou `null`. |
| `GET` | `/api/grades?exam=<chave>` | Retorna uma avaliação. |
| `GET` | `/api/grades/<chave>` | Alias da consulta de uma avaliação. |
| `GET` | `/api/grades/all` | Retorna as avaliações ativas. |
| `POST` | `/api/passkey/register/options` | Inicia o cadastro de passkey. |
| `POST` | `/api/passkey/register` | Conclui o cadastro de passkey. |
| `POST` | `/api/passkey/login/options` | Inicia o login por passkey. |
| `POST` | `/api/passkey/login` | Conclui o login por passkey. |
| `GET` | `/api/docs` | Documentação protegida por Basic Auth. |

As rotas de notas exigem cookie de sessão. O parâmetro `refresh=1` limpa o cache em memória antes de consultar os dados novamente.

## Diagnóstico de comentários

O utilitário usa a mesma configuração da aplicação:

```bash
go run ./cmd/comments
go run ./cmd/comments -matricula 12345678 -exam ab1
go run ./cmd/comments -matricula 12345678 -exam ab1 -sheet "atividade 1"
go run ./cmd/comments -raw-drive
```

## Qualidade

```bash
go mod verify
go vet $(go list ./... | grep -v /node_modules/)
go test -race $(go list ./... | grep -v /node_modules/)
npm run lint
npm run build
npm audit --audit-level=high
```

O workflow `quality-gate` executa essas verificações em pushes e pull requests para `main`, além de validar formatação Go e higiene do repositório.

## Deploy na Vercel

O projeto usa [vercel.json](vercel.json) para gerar o frontend e encaminhar `/api/*` à função Go. Configure no ambiente de produção:

```env
GOOGLE_SHEET_V2_IDS=id_da_planilha_1,id_da_planilha_2
LOGIN_SHEET_NAME=Base de dados
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=json_da_service_account_em_base64
SESSION_SECRET=uma-chave-aleatoria-forte
COOKIE_SECURE=true
DOCS_USERNAME=usuario-da-documentacao
DOCS_PASSWORD=senha-da-documentacao
TURNSTILE_SECRET_KEY=chave-privada
VITE_TURNSTILE_SITE_KEY=chave-publica
```

Mantenha `VITE_API_BASE` vazio quando frontend e API estiverem no mesmo projeto Vercel.

## Documentação adicional

- A documentação da API está em `/api/docs` e `/api/docs?format=json`.
- A documentação operacional mantida no repositório fica em [docs/wiki](docs/wiki).
- O workflow `sync-wiki` publica esse diretório na GitHub Wiki.
