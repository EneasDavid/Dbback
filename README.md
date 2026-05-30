# dbBack
Projeto desenvolvido por David Eneas para a disciplina de Banco de Dados do curso para auxiliar os alunos a terem acesso fácil e rápido às suas notas e feedbacks de atividades, centralizando as informações em uma interface mobile amigável.

Aplicacao Go + React para consulta mobile de notas por matricula em uma planilha do Google Sheets.

## Como funciona

- Login por matricula existente na aba `Base de dados`.
- A aba de login precisa ter uma coluna de matricula e uma coluna de nome.
- Sessao de 7 horas em cookie HTTP-only assinado. Ao expirar, o aluno precisa fazer login novamente.
- Consulta somente leitura via Google Sheets API.
- Selecao entre `AB1` e `AB2`.
- Cada AB pode consultar varias abas/tabelas da mesma planilha.
- Retorno apenas das linhas do nome vinculado a matricula logada.
- Feedbacks das atividades sao lidos dos comentarios/notas das celulas quando a API do Sheets expoe esse dado.
- Atividades retornam a rubrica completa: subtopico, nota maxima, nota alcancada e comentario.

## Arquitetura

O projeto e um monolito modular: uma unica aplicacao deployavel, com responsabilidades separadas por modulo.

- `api/`: rotas HTTP.
- `cmd/dev/`: servidor local integrado.
- `pkg/app/`: regras de sessao, configuracao, Google Sheets, parsing de tabelas e comentarios.
- `src/`: frontend React, separado em componentes, API client, tipos e utilitarios de notas.

## Amplitude tecnica

### Cache e sessao

- Sessao assinada em cookie HTTP-only com validade de 7 horas.
- Ao expirar, `/api/me` e `/api/grades` deixam de autorizar a consulta e o aluno precisa fazer login novamente.
- Ao sair, o cookie e invalidado no backend e o cache local do aluno e apagado no frontend.
- O backend mantem cache em memoria por 7 horas para grids do Google Sheets e comentarios exportados do XLSX.
- O cache usa `singleflight` para evitar multiplas chamadas simultaneas iguais ao Google Sheets/Drive.
- No frontend, as notas ficam em `sessionStorage` apenas durante a sessao da aba; reload força refresh da API quando necessario.

### Seguranca e privacidade

- A consulta e somente leitura, usando escopo read-only do Google Sheets.
- O login aceita apenas matricula e valida a correlacao matricula/nome na aba de base.
- A resposta retorna apenas a linha vinculada ao aluno logado.
- Cookies usam assinatura HMAC, `HttpOnly`, `SameSite=Lax` e `Secure` quando habilitado em producao.
- A API envia `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` e `Permissions-Policy` restritivo.
- Segredos ficam em variaveis de ambiente ou arquivo local ignorado pelo Git.

### Rede de computadores

- O frontend conversa com a API do mesmo dominio, reduzindo CORS e superficie de exposicao.
- A API atua como proxy de leitura controlado entre o aluno e Google Sheets/Drive.
- O cache reduz latencia, consumo de banda e risco de rate limit no Google.
- Requisicoes AB1 e AB2 sao carregadas em paralelo para melhorar tempo percebido.
- Em producao, recomenda-se HTTPS, `COOKIE_SECURE=true` e rate limit no proxy/edge.

### Estruturas de dados, grafos e PAA

- As planilhas sao normalizadas para uma estrutura tabular em memoria: cabecalhos, linhas, notas e autores.
- Celulas mescladas sao expandidas antes do parsing para preservar a relacao grupo -> aluno -> nota.
- A relacao `AB -> atividade -> subtopico -> nota maxima -> nota alcancada -> comentario` forma uma arvore de avaliacao.
- Comentarios sao indexados por referencia de celula, funcionando como um mapa `celula -> comentario`.
- A correlacao matricula/nome usa busca linear sobre a aba base; para turmas maiores pode evoluir para indice `map[matricula]aluno`.
- O custo principal por aba e `O(L * C)`, onde `L` e quantidade de linhas e `C` quantidade de colunas. A expansao de merges e `O(M * A)`, com `M` merges e `A` area mesclada.
- O projeto aplica ideias de PAA ao reduzir chamadas externas, evitar recomputacao com cache e preservar ordem deterministica das tabelas.

### POO e arquitetura de software

- O backend Go usa tipos com responsabilidade clara, como `SheetsClient`, `SessionManager`, `Config`, `GradeResult`, `TableResult` e `ActivityItem`.
- O design segue separacao de responsabilidades: configuracao, sessao, HTTP, login, parsing, normalizacao, comentarios e grades.
- O frontend separa componentes visuais, cliente HTTP, tipos e regras de nota.
- A arquitetura favorece baixo acoplamento: mudancas na origem dos dados ficam em `pkg/app`, enquanto regras de exibicao ficam em `src`.
- O monolito modular facilita deploy simples sem abrir mao de organizacao interna.

## Variaveis de ambiente

Crie as variaveis no ambiente onde o projeto for rodar:

- `GOOGLE_SHEET_ID`: id da planilha.
- `LOGIN_SHEET_NAME`: aba com a base de matriculas.
- `SHEET_AB1_PESQUISA`: aba da pesquisa AB1.
- `SHEET_AB1_ARTIGO`: aba do artigo AB1.
- `SHEET_AB1_LISTA`: aba da lista AB1.
- `SHEET_AB1_PROVA`: aba da prova/notas AB1.
- `SHEET_AB2_LISTA`: aba da lista AB2.
- `SHEET_AB2_PROJETO`: aba do projeto AB2.
- `SESSION_SECRET`: chave longa e aleatoria para assinar a sessao.
- `COOKIE_SECURE`: `true` em producao, `false` em dev local sem HTTPS.
- `GOOGLE_SERVICE_ACCOUNT_JSON`: JSON completo da service account.
- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`: alternativa ao JSON direto, boa para CI.
- `GOOGLE_SERVICE_ACCOUNT_FILE`: caminho local para o arquivo JSON da service account.
- `GOOGLE_SHEET_XLSX_FILE`: opcional; caminho local para um `.xlsx` exportado com threaded comments.

Compartilhe a planilha com o e-mail `client_email` da service account como leitor.

Nas abas de notas, o sistema procura primeiro uma coluna de nome (`Nome`, `Aluno`, `Estudante`, etc.).
Se a aba nao tiver nome, usa matricula como fallback.

## Desenvolvimento

```bash
npm install
go mod download
npm run dev:full
```

O comando `dev:full` compila o frontend e sobe a API Go junto em `http://localhost:8080`.

`npm run dev` continua disponivel para abrir apenas o frontend Vite.

## Deploy

O workflow `.github/workflows/ci.yml` roda somente lint, build e testes Go. Deploy fica opcional e fora do CI padrao, sem exigir login ou token de provedor.
