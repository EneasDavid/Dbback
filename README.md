# dbBack

Monolito Go + React para consulta autenticada de notas e feedbacks a partir de Google Sheets. A aplicacao valida a matricula na aba de login, resolve a identidade do aluno, le somente a linha correspondente nas abas avaliativas e entrega um payload pronto para a UI.

## Documentacao da API

A documentacao de rotas, schemas e organizacao do payload fica na propria API:

- Local: `GET /api/docs`
- Vercel: <https://dbback.vercel.app/api/docs>

A rota de docs usa Basic Auth. O usuario e a senha existem somente no ambiente da aplicacao; nao registre valores de login no README.

### Endpoints HTTP

Os endpoints de sessao e documentacao respondem com `Cache-Control: no-store`. As rotas de notas usam cache privado por usuario com `ETag`, `Cache-Control: private, max-age=30, stale-while-revalidate=300` e `Vary: Cookie, Accept-Encoding`. Em Vercel, o roteador tambem aceita o prefixo serverless `/api/index.go/` para as mesmas rotas.

| Metodo | Caminho | Auth | Descricao |
| --- | --- | --- | --- |
| `GET` | `/api`, `/api/index`, `/api/index.go` | Basic Auth docs | Alias para a documentacao HTML da API. |
| `GET` | `/api/docs` | Basic Auth docs | Documentacao HTML quando `Accept` pede HTML. |
| `GET` | `/api/docs?format=json` | Basic Auth docs | Documentacao JSON com rotas, schemas, organizacao de dados e rede/performance. |
| `POST` | `/api/login` | Nao | Recebe `{ "matricula": "..." }`, valida a aba `Base de dados`, cria cookie de sessao e fixa a planilha internamente com `schemaStatus` publico. |
| `POST` | `/api/logout` | Nao | Limpa o cookie de sessao. |
| `GET` | `/api/me` | Cookie opcional | Retorna o usuario da sessao ou `null`. |
| `GET` | `/api/grades?exam=<avaliacao>` | Cookie | Retorna uma avaliacao. No legado use `ab1`/`ab2`; na v2 use a chave ativa da aba `abs`. |
| `GET` | `/api/grades/<avaliacao>` | Cookie | Alias path-based de `/api/grades?exam=<avaliacao>`. |
| `GET` | `/api/grades/all` | Cookie | Retorna todas as avaliacoes disponiveis. Na v2 retorna somente ABs com `status`/ativo igual a `1` na aba `abs`. |

Query comum de notas: `refresh=1` limpa o cache em memoria do processo antes de ler dados.

## Arquitetura

O projeto segue uma organizacao MVC. No frontend, a arvore fisica separa explicitamente responsabilidades:

- `src/Models/`: tipos, normalizacao de payloads, cache, compatibilidade legado/V2 e flag `v2_stable: true`.
- `src/Views/`: componentes React e estilos da interface.
- `src/Controllers/`: controle de fluxo da aplicacao, sessoes, chamadas HTTP e coordenacao entre Models e Views.

No backend Go, a separacao continua organizada por responsabilidade:

- `api/`: roteador serverless, controllers HTTP e view HTML das docs.
- `pkg/app/`: modelos, configuracao, sessoes, servicos de notas e repositorio Google Sheets/Drive.
- `cmd/dev/`: servidor local que serve frontend e API no mesmo processo.
- `cmd/comments/`: utilitario de diagnostico para comentarios vistos pela service account.
- `src/`: raiz do frontend MVC (`Models`, `Views`, `Controllers`).

Fluxo principal:

```text
React/Vite -> api Router -> Controllers -> pkg/app services -> Google Sheets/Drive
```

Os comentarios ricos do Google Drive sao usados apenas como enriquecimento. Se a Drive API ou os comentarios falharem, a leitura dos valores da planilha via Sheets continua funcionando.

## Otimizacoes tecnicas

- Meta de UX: no caminho quente, login + primeiro render de notas deve ficar em ate 2s. Caminho quente significa runtime Vercel ativo, cache do servidor dentro do TTL e/ou cache SWR local no navegador.
- `GET /api/grades/all` carrega todas as avaliacoes disponiveis em uma unica chamada HTTP do frontend.
- `POST /api/login` inicia um warm-up assíncrono de `GradesFor` logo depois de criar a sessao; a chamada seguinte de notas compartilha cache/singleflight quando cai no mesmo runtime.
- Na v2, `GET /api/grades/all` carrega apenas ABs ativas da aba `abs`, evitando payload e UI para avaliacoes indisponiveis.
- O backend agrega as abas configuradas e faz uma unica leitura em lote no Google Sheets sempre que possivel.
- Valores, notas de celula e metadados de merges sao buscados juntos pelo Sheets API com `Fields` restrito.
- Comentarios ricos do Drive e comentarios exportados do XLSX sao buscados em paralelo com a leitura das abas e aplicados como enriquecimento quando a celula pode ser mapeada.
- Comentarios em celulas de criterio/nota entram no mesmo payload das notas, sem requisicao extra do frontend.
- Cache em memoria por planilha/aba com TTL reduz chamadas repetidas ao Google durante a mesma janela de uso.
- `singleflight` evita chamadas duplicadas quando varias requisicoes pedem as mesmas abas ao mesmo tempo.
- Escopos de planilha compartilham o mesmo runtime de cache/singleflight, sem misturar abas homonimas de planilhas diferentes.
- As rotas de notas retornam `ETag` e aceitam `If-None-Match`; respostas sem alteracao voltam como `304 Not Modified`.
- O frontend aplica SWR: renderiza `sessionStorage` imediatamente, respeita `max-age` do cache HTTP, preserva dados locais em `304` e deduplica GETs em voo.
- Cards de atividade fazem prefetch das notas em `hover` e `focus`, antecipando a rede antes da expansao.
- O cliente Google usa timeout total, timeout de handshake/cabecalho, keep-alive e pool de conexoes para impedir que chamadas lentas prendam a API e para reduzir latencia em rajadas.
- O Vercel entrega payloads textuais com compressao Brotli/Gzip conforme `Accept-Encoding`; a API sinaliza `Vary: Accept-Encoding` nas notas.
- Abas de controle v2 (`abs` e `atividades`) nao disparam export/busca de comentarios Drive/XLSX, reduzindo trafego antes da leitura das notas.
- O export XLSX usado para comentarios e limitado a 25 MiB.
- O frontend guarda o payload de notas em `sessionStorage`; alternar entre avaliacoes nao dispara nova chamada de rede.
- A UI usa o payload normalizado do backend e nao recalcula regras sensiveis de nota no navegador.

## PAA, seguranca e integridade

PAA aqui significa Plano de Acesso e Auditoria: cada requisicao deve validar acesso, escolher a fonte correta e devolver um payload rastreavel sem expor segredos.

- Plano: `GOOGLE_SHEET_IDS` define todas as bases ativas. A API consulta as bases em ordem, prende a sessao ao `spreadsheetId` encontrado e, se aquela base nao tiver notas renderizaveis, tenta a proxima antes de devolver vazio.
- Acesso: login e notas usam apenas a service account configurada; a planilha precisa estar compartilhada com o `client_email`. Arquivos JSON locais ficam no `.gitignore`; em deploy use `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`.
- Auditoria: o payload retorna `schemaStatus` quando a origem e conhecida; o `spreadsheetId` fica apenas na sessao assinada do servidor e nao e exposto nas respostas publicas.
- Integridade: a sessao usa HMAC-SHA256 e respostas JSON recebem digest SHA-256, tecnicas semelhantes ao principio de hash encadeado usado em blockchains para detectar alteracao. O sistema nao grava dados em blockchain publica; aplica o conceito util aqui: token assinavel, verificavel e resistente a adulteracao.
- Disponibilidade: erros de uma planilha inacessivel em configuracoes com multiplas bases nao derrubam todo o fluxo quando outra base valida pode responder.

Grafo de fluxo principal:

```text
LoginView -> POST /api/login -> AuthController -> LoginIdentity(Base de dados)
AuthController -> warm-up GradesFor -> SheetsClient cache/singleflight
AppController -> GET /api/grades/all + If-None-Match -> GradesController
GradesController -> SheetsClient -> Google Sheets/Drive
GradesController -> JSON privado + ETag -> GradeModel -> GradeCard/DetailPanel
```

## Configuracao

Copie `env.example` para `.env` no desenvolvimento local e configure as variaveis do projeto. As principais sao:

```env
GOOGLE_SHEET_ID=...
# Compatibilidade antiga: lista mista opcional.
# GOOGLE_SHEET_IDS=id_da_turma_1,id_da_turma_2
# Preferido para bases separadas por versao:
# GOOGLE_SHEET_LEGACY_IDS=id_legado_1,id_legado_2
# GOOGLE_SHEET_V2_IDS=id_v2_1,id_v2_2
# SHEETS_RUNTIME_VERSION=auto # auto, legacy ou v2
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

A tag git local `v1-stable` aponta para o codigo estavel anterior a v2. Em runtime, `GOOGLE_SHEET_ID` e `GOOGLE_SHEET_IDS` continuam funcionando como configuracao antiga/mista. Para deixar varias planilhas online ao mesmo tempo e evitar tentativa desnecessaria do parser errado, prefira `GOOGLE_SHEET_LEGACY_IDS` para bases legadas e `GOOGLE_SHEET_V2_IDS` para bases v2, com IDs separados por virgula, ponto e virgula ou quebra de linha. Se mais de uma variavel estiver definida, o backend deduplica e consulta todas em ordem: legado primeiro, depois v2, depois mistas.

Quando `SHEETS_RUNTIME_VERSION=v2`, a API consulta os metadados do proprio Google Sheets. A planilha e marcada como `v2` quando houver developer metadata com a chave `GOOGLE_SHEET_METADATA_KEY` e o valor `GOOGLE_SHEET_METADATA_VALUE`; qualquer divergencia fica marcada como `legacy` no payload.

Na v2, as atividades nao saem mais da lista fixa `SHEET_AB1_*`. O backend le a aba `abs` para descobrir quais ABs estao ativas, le a aba `atividades` para descobrir as atividades de cada AB e seu `peso maximo`, le `nota <ab>` para a media e a nota final do aluno em cada atividade, e entao abre a aba da atividade para montar os criterios, grupos/matriculas e comentarios por subtopico. Somente linhas da aba `abs` com `status`/ativo igual a `1` entram em `/api/grades/all`.

#### Tópicos (Critérios de Aceite) e Comentários na V2

A v2 retorna cada critério de aceite como um tópico (Detail) dentro do card de atividade, com:
- **Label**: nome do critério (do cabeçalho da coluna)
- **Value**: nota alcançada pelo aluno
- **Max**: nota máxima do critério
- **Comment**: feedback do professor por critério
- **CommentAuthor**: nome ou cargo de quem escreveu o feedback

Os comentários são colhidos automaticamente das notas de células do Google Sheets (cell notes / workbook comments). Se um critério não tiver comentário, o campo fica vazio.

Na v2, a nota principal do card da atividade vem da aba `nota <ab>`; se a coluna da atividade nao casar pelo nome, a API tenta a coluna `nota final` nessa mesma aba de resumo. A coluna `nota final` da aba da atividade nao aparece como criterio e nao entra no calculo das notas maximas dos criterios. Medias sao limitadas a 10 pontos no payload.

Mesmo com `SHEETS_RUNTIME_VERSION=v2`, o parser legado continua disponivel como fallback. Se a estrutura v2 nao existir, se a AB estiver sem tabelas v2 renderizaveis ou se a planilha ainda estiver no formato antigo, a mesma requisicao tenta o fluxo legado configurado por `SHEET_AB1_*`/`SHEET_AB2_*`.

No login, a API procura a matricula em todas as planilhas configuradas e salva o `spreadsheetId` de origem somente na sessao assinada. As consultas de notas seguintes ficam presas a esse mesmo arquivo, evitando misturar dados da planilha antiga com os da nova sem expor o ID da planilha ao frontend.

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
