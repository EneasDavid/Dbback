# dbBack

Aplicação Go + React para consulta mobile de notas e feedbacks por matrícula, usando uma planilha Google Sheets como fonte read-only. O foco do projeto é acesso rápido em celular, inclusive em conexão ruim, sem expor a planilha nem dados de outros alunos.

## Produto

- Login por matrícula validada na aba `Base de dados`.
- Sessão de 7 horas em cookie assinado, `HttpOnly`, `SameSite=Lax` e `Secure` em produção.
- Consulta individual de AB1 e AB2.
- Feedbacks lidos online pela Google Sheets API (`note`) e pela Google Drive API (`comments`) quando forem comentários ricos.
- Payload render-ready: o backend retorna `tables[].cards[].details[]`, reduzindo cálculo no aparelho.
- Cache no backend e no navegador para diminuir latência, tráfego e risco de rate limit.

## Arquitetura

O projeto é um monolito modular com frontend estático e API Go:

- `api/`: rotas HTTP (`/api/login`, `/api/logout`, `/api/me`, `/api/grades`).
- `cmd/dev/`: servidor local que serve `dist/` e proxy da API no mesmo processo.
- `pkg/app/`: sessão, configuração, cliente Google Sheets, parsing, normalização e regras de renderização.
- `src/`: React, API client, tipos e componentes visuais.

Fluxo principal:

```text
Aluno -> React -> /api/login -> Google Sheets Base de dados
Aluno -> React -> /api/grades?exam=ab1|ab2 -> SheetsClient -> Google Sheets
Google Sheets/Drive -> valores + notes/comments -> parser Go -> cards/details -> React
```

## Dados e regras

- A service account usa `spreadsheets.readonly` e `drive.readonly`.
- O backend busca abas em lote por exame, deduplica nomes de abas e aplica `Fields(...)` para retornar só `formattedValue`, `userEnteredValue`, `note`, `merges`, `properties.title` e `properties.sheetId`.
- A Google Sheets API retorna notas de célula (`note`). Comentários ricos/threaded são consultados pela Google Drive API e associados ao grid da aba.
- A planilha é normalizada em memória como `sheetGrid`: cabeçalhos, linhas, notas e índices reais.
- Células mescladas são expandidas antes do parsing.
- Comentários dos subtópicos seguem esta precedência:
  1. nota da célula do aluno;
  2. nota da linha do subtópico;
  3. nota do cabeçalho.
- Médias pendentes não são renderizadas como card de média.
- A resposta nunca retorna lista completa de alunos; somente dados compatíveis com a sessão.

## Rede e performance

- O frontend usa o mesmo domínio da API quando possível, reduzindo CORS e cookies cross-site.
- `/api/*` envia `Cache-Control: no-store` porque contém dados privados.
- Assets em `/assets/*` usam cache imutável por hash.
- O backend mantém cache em memória por 7 horas e usa `singleflight` para evitar chamadas duplicadas ao Google.
- O frontend usa `sessionStorage` versionado para mostrar dados imediatamente em reload.
- Em redes `saveData`, `slow-2g` ou `2g`, o app busca primeiro só a AB ativa; em conexões melhores, pré-carrega a outra AB em segundo plano.

## Segurança

- Sem endpoint para listar usuários ou consultar matrícula arbitrária autenticada.
- Cookie assinado por HMAC com `SESSION_SECRET`.
- Headers de resposta: `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy` e `Cache-Control`.
- Segredos devem ficar em variáveis de ambiente. Não versionar JSON de service account, planilhas locais, `.pem`, `.key`, patches ou binários.
- O CI rejeita arquivos gerados ou secret-like rastreados.

## Complexidade e estrutura de dados

- Parsing por aba: `O(L * C)`, com `L` linhas e `C` colunas.
- Expansão de merges: `O(M * A)`, com `M` ranges mesclados e `A` área total expandida.
- Busca de aluno: linear por aba, adequada para turmas pequenas/médias. Para turmas maiores, o próximo passo natural é índice `map[matricula]linha`.
- A árvore lógica é `AB -> tabela -> card -> detalhe -> comentário`.

## Variáveis de ambiente

```env
GOOGLE_SHEET_ID=...
LOGIN_SHEET_NAME=Base de dados
SHEET_AB1_PESQUISA=AT. 1
SHEET_AB1_ARTIGO=AT. 2
SHEET_AB1_LISTA=AT. 3
SHEET_AB1_PROVA=Notas AB1
SHEET_AB2_LISTA=AT. 4
SHEET_AB2_PROJETO=Projeto AB2
SESSION_SECRET=use-uma-chave-forte-com-mais-de-32-caracteres
COOKIE_SECURE=true
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=...
```

Alternativas para credenciais:

- `GOOGLE_SERVICE_ACCOUNT_JSON`: JSON completo.
- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`: recomendado para CI/deploy.
- `GOOGLE_SERVICE_ACCOUNT_FILE`: apenas para desenvolvimento local.

Compartilhe a planilha com o `client_email` da service account. Para comentários ricos, use permissão de **Comentador** ou **Editor** para garantir que a Drive API consiga listar os comentários visíveis.

Os feedbacks das células são carregados em produção pelas APIs do Google. O Vercel não depende de planilha local.

### Vercel

No Vercel, nao use `GOOGLE_SERVICE_ACCOUNT_FILE`: arquivos `.json` locais sao ignorados pelo Git e nao chegam ao deploy. Gere a credencial em base64 e cadastre como variável do projeto:

```bash
base64 < service-account.local.json | tr -d '\n'
```

Configure no painel do Vercel, em Production e Preview quando precisar testar:

```env
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<saida-do-comando-base64>
GOOGLE_SHEET_ID=<id-da-planilha>
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
```

Deixe `VITE_API_BASE` vazio quando o frontend e `/api/*` estiverem no mesmo projeto Vercel.

## Desenvolvimento

```bash
npm install
go mod download
npm run dev:full
```

Dependência usada para carregar `.env` no utilitário local:

```bash
go get github.com/joho/godotenv
```

Para verificar, com as mesmas variáveis de ambiente do app, quais comentários estão chegando pela service account:

```bash
go run ./cmd/comments
```

Esse comando usa `GOOGLE_SERVICE_ACCOUNT_FILE` ou `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`, `GOOGLE_SHEET_ID` e as variáveis `SHEET_*` do `.env`, consulta as APIs do Google e imprime célula e texto de cada nota/feedback.

Se o comando retornar zero feedbacks, a própria service account não está enxergando notas/comentários nas abas configuradas; nesse caso o Vercel também não terá como renderizá-los até que esses feedbacks estejam na planilha online acessível por essa credencial.

Comandos de qualidade:

```bash
go vet $(go list ./... | grep -v /node_modules/)
go test -race $(go list ./... | grep -v /node_modules/)
npm run lint
npm run build
npm audit --audit-level=high
```

O hook local de pre-push já está versionado em `.githooks/pre-push`. Para ativar em outro clone:

```bash
git config core.hooksPath .githooks
```

## CI/CD

O workflow `.github/workflows/ci.yml` roda em push e pull request para `main`:

- higiene do repositório e formatação Go;
- `go mod verify`;
- `go vet`;
- `go test -race`;
- `govulncheck`;
- `npm ci`;
- `npm run lint`;
- `npm run build`;
- `npm audit --audit-level=high`.

Para impedir merge quando algo falhar, habilite no GitHub:

- branch protection em `main`;
- required status check: `required quality gate`;
- bloqueio de direct push em `main`;
- pull request obrigatório antes de merge.

## Commits

Use mensagens descritivas no padrão Conventional Commits:

```text
feat: render grades from Sheets notes
fix: ignore legacy grade cache without cards
ci: add required quality gate
docs: consolidate architecture and security notes
```

Antes de publicar, rode a qualidade local ou deixe o pre-push bloquear o envio.
