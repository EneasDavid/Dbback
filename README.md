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
