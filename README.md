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

## Otimizacoes tecnicas

- `GET /api/grades/all` carrega AB1 e AB2 em uma unica chamada HTTP do frontend.
- O backend agrega as abas configuradas e faz uma unica leitura em lote no Google Sheets sempre que possivel.
- Valores, notas de celula e metadados de merges sao buscados juntos pelo Sheets API com `Fields` restrito.
- Comentarios ricos do Drive sao buscados em paralelo com a leitura das abas e aplicados como enriquecimento.
- Comentarios em celulas de criterio/nota entram no mesmo payload das notas, sem requisicao extra do frontend.
- Cache em memoria por aba com TTL reduz chamadas repetidas ao Google durante a mesma janela de uso.
- `singleflight` evita chamadas duplicadas quando varias requisicoes pedem as mesmas abas ao mesmo tempo.
- O frontend guarda AB1 e AB2 em `sessionStorage`; alternar entre avaliacoes nao dispara nova chamada de rede.
- A UI usa o payload normalizado do backend e nao recalcula regras sensiveis de nota no navegador.

## Configuracao

Copie `env.example` para `.env` no desenvolvimento local e configure as variaveis do projeto. As principais sao:

```env
GOOGLE_SHEET_ID=...
# Para v2/multiplas planilhas, use GOOGLE_SHEET_IDS no lugar de GOOGLE_SHEET_ID:
# GOOGLE_SHEET_IDS=id_da_turma_1,id_da_turma_2
# SHEETS_RUNTIME_VERSION=v2
# GOOGLE_SHEET_METADATA_KEY=dbback_schema
# GOOGLE_SHEET_METADATA_VALUE=v2
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

Nao envie arquivos fisicos de service account para o GitHub. O `.gitignore` bloqueia JSONs locais de credencial, incluindo `service-account*.json` e `spheric-radio-*.json`; no deploy, prefira `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`.

Compartilhe a planilha com o `client_email` da service account. Para comentarios ricos, habilite a Drive API no projeto Google Cloud; sem ela, os valores da planilha ainda sao lidos. Se um comentario aparece no navegador mas nao no diagnostico, verifique se a service account consegue ver esse comentario e se ele esta na mesma linha/celula do aluno ou do criterio avaliado.

### V1 legado e V2

A tag git local `v1-stable` aponta para o codigo estavel anterior a v2. Em runtime, `GOOGLE_SHEET_ID` continua funcionando como configuracao v1/legado. Para deixar varias planilhas online ao mesmo tempo, configure `GOOGLE_SHEET_IDS` com os IDs separados por virgula, ponto e virgula ou quebra de linha.

Quando `SHEETS_RUNTIME_VERSION=v2`, a API consulta os metadados do proprio Google Sheets. A planilha e marcada como `v2` quando houver developer metadata com a chave `GOOGLE_SHEET_METADATA_KEY` e o valor `GOOGLE_SHEET_METADATA_VALUE`; qualquer divergencia fica marcada como `legacy` no payload.

### Gerar o JSON da service account

O site correto e o Google Cloud Console: <https://console.cloud.google.com/>. Use o mesmo projeto do arquivo esperado, por exemplo `spheric-radio-495913-q2`.

1. Entre em <https://console.cloud.google.com/> e selecione o projeto.
2. Ative as APIs no projeto:
   - Google Sheets API: <https://console.cloud.google.com/apis/library/sheets.googleapis.com>
   - Google Drive API: <https://console.cloud.google.com/apis/library/drive.googleapis.com>
3. Abra IAM e administrador > Contas de servico, ou entre direto em <https://console.cloud.google.com/iam-admin/serviceaccounts>.
4. Clique em Criar conta de servico, informe um nome como `dbback-sheets-reader` e finalize.
5. Abra a conta criada, va em Chaves > Adicionar chave > Criar nova chave, escolha JSON e baixe o arquivo.
6. Para desenvolvimento local, renomeie o arquivo baixado para `spheric-radio-495913-q2-1fd5fc001597.json` e use:

```env
GOOGLE_SERVICE_ACCOUNT_FILE=./spheric-radio-495913-q2-1fd5fc001597.json
```

7. Abra a planilha no Google Sheets, clique em Compartilhar e adicione o `client_email` que aparece dentro do JSON como leitor da planilha.

Para Vercel/GitHub, nao envie esse JSON. Gere base64 e salve somente como variavel de ambiente:

```bash
base64 < spheric-radio-495913-q2-1fd5fc001597.json | tr -d '\n'
```

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
