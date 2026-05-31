# dbBack

Monolito Go + React para consulta autenticada de notas e feedbacks a partir de Google Sheets e Google Drive. A aplicação valida a matrícula na aba `Base de dados`, resolve a identidade do aluno, lê somente a linha correspondente nas abas avaliativas e devolve um payload pronto para renderização mobile.

## Arquitetura Técnica

```text
React/Vite
  -> /api/login
  -> /api/me
  -> /api/grades?exam=ab1|ab2
  -> /api/docs

Go HTTP handler
  -> SessionManager
  -> SheetsClient
  -> grid parser
  -> activity/student parsers
  -> render rules
  -> JSON render-ready

Google Sheets API
  -> valores, notas de celula, merges e metadados de abas

Google Drive API
  -> comentarios ricos visiveis para a service account
```

Diretorios principais:

- `api/`: roteamento HTTP serverless/monolito.
- `cmd/dev/`: servidor local que serve frontend e API no mesmo processo.
- `cmd/comments/`: utilitario de diagnostico dos comentarios vistos pela service account.
- `pkg/app/`: configuracao, sessoes, cliente Sheets/Drive, parsing, normalizacao e regras de renderizacao.
- `src/`: UI React, cliente HTTP, tipos e componentes.

## Rotas HTTP

A documentacao online das rotas esta disponivel em:

```text
GET /api/docs
```

Essa rota usa Basic Auth para funcionar corretamente em producao/Vercel sem depender da sessao do aluno:

```text
usuario: adão
senha: primeiro
```

Rotas expostas:

- `POST /api/login`: recebe `{ "matricula": "..." }`, valida em `Base de dados` e cria cookie assinado.
- `POST /api/logout`: remove cookie de sessao.
- `GET /api/me`: retorna o aluno autenticado ou `null`.
- `GET /api/grades?exam=ab1|ab2`: retorna notas e feedbacks da avaliacao solicitada; exige sessao.
- `GET /api/grades/exam=ab1|ab2`: alias para a rota de notas; exige sessao.
- `GET /api/grades/ab1|ab2`: alias curto para a rota de notas; exige sessao.
- `GET /api/docs`: abre uma página HTML com metadados tecnicos das rotas e da organizacao dos dados; exige Basic Auth.
- `GET /api/docs?format=json`: retorna os mesmos metadados em JSON; exige Basic Auth.

Formato dos JSONs, apenas com campos e tipos:

```json
POST /api/login -> {
  "matricula": "string",
  "name": "string"
}

POST /api/logout -> {
  "ok": "boolean"
}

GET /api/me -> {
  "matricula": "string",
  "name": "string"
}

GET /api/me, sem sessao -> null

GET /api/docs?format=json -> {
  "name": "string",
  "type": "string",
  "description": "string",
  "routes": "array<object>",
  "gradeOrganization": "object"
}
```

## Organizacao de Notas e Comentarios

A unidade de leitura e a linha do aluno:

1. `/api/login` procura a matrícula na aba `Base de dados`.
2. A matrícula resolve o `nome` canonico da sessao.
3. Para cada aba de AB1/AB2, o parser procura a linha cuja celula de identidade corresponde ao nome ou à matrícula.
4. Celulas de identidade sao avaliadas nesta ordem: `Nome`, `Grupo`/`Equipe`, `Matrícula`, primeira coluna.
5. O feedback renderizado vem somente da celula de identidade da linha selecionada.
6. As demais celulas da mesma linha viram valores estruturados em `tables[].cards[]` e `cards[].details[]`.
7. Se a celula de identidade contiver uma indicacao como `NOME NAO CONSTA NA ATIVIDADE`, a linha e ignorada e nao retorna dados de nota.

Essa regra evita associar feedback de outra linha ou de outra aba por coincidencia de nota numerica. Comentarios em criterios individuais deixam de ser fonte primaria; o comentario correto deve ficar na celula de identidade da linha do aluno.

## Modelo de Dados

Resposta de `/api/grades?exam=ab1|ab2`, `/api/grades/exam=ab1|ab2` e `/api/grades/ab1|ab2`:

```ts
type GradeResult = {
  exam: string;
  matricula: string;
  name: string;
  tables: GradeTable[];
  studentStatus?: StudentStatus;
};

type GradeTable = {
  key: string;
  label: string;
  sheetName: string;
  kind: string;
  complete: boolean;
  status?: string;
  cards: GradeCard[];
};

type GradeCard = {
  key: string;
  label: string;
  value: string;
  displayValue: string;
  tone?: string;
  comment?: string;
  commentAuthor?: string;
  details?: GradeDetail[];
};

type GradeDetail = {
  key: string;
  label: string;
  value: string;
  max: number;
  displayScore: string;
  ratio: number;
  pending: boolean;
  tone?: string;
  comment?: string;
  commentAuthor?: string;
};

type StudentStatus = {
  ab1: number;
  ab2: number;
  average: number;
  approved: boolean;
};
```

Exemplo estrutural sem dados reais:

```json
{
  "exam": "string",
  "matricula": "string",
  "name": "string",
  "tables": [
    {
      "key": "string",
      "label": "string",
      "sheetName": "string",
      "kind": "string",
      "complete": "boolean",
      "status": "string opcional",
      "cards": [
        {
          "key": "string",
          "label": "string",
          "value": "string",
          "displayValue": "string",
          "tone": "string opcional",
          "comment": "string opcional",
          "commentAuthor": "string opcional",
          "details": [
            {
              "key": "string",
              "label": "string",
              "value": "string",
              "max": "number",
              "displayScore": "string",
              "ratio": "number",
              "pending": "boolean",
              "tone": "string opcional",
              "comment": "string opcional",
              "commentAuthor": "string opcional"
            }
          ]
        }
      ]
    }
  ],
  "studentStatus": "object opcional"
}
```

O backend preserva a privacidade retornando apenas o recorte da linha associada à sessao. O frontend nao calcula regras de nota sensiveis; ele apenas renderiza o JSON normalizado.

## Tecnicas Implementadas

Redes de computadores:

- HTTP stateless com cookies assinados por HMAC.
- `Cache-Control: no-store` em dados privados.
- Reuso de cliente OAuth2 para Google APIs.
- `singleflight` para reduzir chamadas duplicadas em rajadas concorrentes.

Projeto e analise de algoritmos:

- Parsing de grade em `O(L * C)`, onde `L` e o numero de linhas e `C` o numero de colunas.
- Expansao de merges em `O(M * A)`, com `M` ranges mesclados e `A` area total expandida.
- Ordenacao estavel de cards de resumo por prioridade sem alterar dados originais.
- Corte antecipado quando a matrícula/nome nao aparece na aba.

Estruturas de dados:

- Matrizes `[][]string` para valores e notas de celulas.
- Maps de deduplicacao para abas, colunas e ranges.
- Estruturas imutaveis no payload final: `GradeResult`, `TableResult`, `CardResult`, `DetailResult`.
- Cache em memoria por aba com TTL.

Grafos e representacao hierarquica:

- A resposta e uma arvore `avaliacao -> tabela -> card -> detalhe`.
- A UI expande detalhes por chave de tabela/card, mantendo estado local minimalista.
- As dependencias de parsing seguem um fluxo aciclico: Sheets/Drive -> grid -> parser -> render rules -> JSON.

POO e modularidade:

- `SheetsClient` encapsula credenciais, cache, chamadas Google e carregamento de abas.
- `SessionManager` encapsula assinatura e leitura do cookie.
- Parsers usam Strategy por `kind` de tabela: `activity`, `summary`, `ab2summary` e `project`.
- Regras de media usam Strategy por avaliacao, com `scoreAverageRule` para AB1 e AB2.
- A coleta de comentarios Drive usa um Template Method para compartilhar paginacao entre API v3 e fallback v2.
- Parsers ficam separados por dominio: atividade rubricada, tabela de aluno, projeto e resumo.
- Regras de exibicao ficam isoladas em `render_rules.go`.

UI e UX:

- Interface mobile-first em React.
- Segmento AB1/AB2, cards de nota, paineis expansíveis e feedback inline.
- Tema claro/escuro persistido no navegador.
- Estado vazio temporizado para evitar piscada durante carregamento.
- A média e sempre renderizada por ultimo usando o mesmo componente de resumo.
- O frontend normaliza o contrato atual `tables[].cards[]` e ainda entende payload legado com `columns`/`items` para evitar tela vazia em builds fora de sincronia.

Ciencia de dados:

- Normalizacao de identificadores, nomes e cabecalhos.
- Tratamento de notas pendentes, medias e tons de desempenho.
- Derivacao de somatorio/media com teto em 10 quando aplicavel.
- Divisor de escala por tabela: subtópicos das atividades AB1 e da `AT. 4` da AB2 usam divisor `10` antes de ir para `cards[].details[]`; o projeto AB2 permanece sem divisor.
- Conversao de valores de planilha para payload tipado e comparavel.

Tecnicas de scraper/coleta:

- A coleta nao usa scraping HTML; usa APIs oficiais do Google.
- O cliente restringe campos (`Fields`) para reduzir payload e superficie de dados.
- Comentarios ricos do Drive sao lidos como fonte auxiliar, mas a associacao exibida usa a celula de identidade da linha.
- O utilitario `cmd/comments` audita o que a service account consegue enxergar.

## Configuracao

Variaveis principais:

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
DOCS_USERNAME=adão
DOCS_PASSWORD=primeiro
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=...
```

Credenciais aceitas:

- `GOOGLE_SERVICE_ACCOUNT_JSON`
- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`
- `GOOGLE_SERVICE_ACCOUNT_FILE` apenas em desenvolvimento local

Compartilhe a planilha com o `client_email` da service account. Para comentarios ricos, a Drive API precisa estar habilitada no projeto Google Cloud.

## Desenvolvimento

```bash
npm install
go mod download
npm run dev:full
npm run production:full
```

O servidor local usa `PORT` e, quando a variavel nao e definida, sobe na porta `3000`.
Nao publique ou mantenha ambientes locais nas portas antigas de desenvolvimento.

Comandos de qualidade:

```bash
go test ./...
npm run lint
npm run build
```

O quality gate do GitHub tambem valida `git diff --check`, `gofmt`, `go mod verify` e rejeita sobras geradas ou arquivos com cara de segredo.

Diagnostico de comentarios:

```bash
go run ./cmd/comments
go run ./cmd/comments -matricula 2024001339 -exam ab1
go run ./cmd/comments -raw-drive
GOOGLE_SERVICE_ACCOUNT_FILE=./service-account.local.json PORT=3000 bash test-comments.sh
```

O `test-comments.sh` usa `GOOGLE_SERVICE_ACCOUNT_FILE` ou `./service-account.local.json`, mascara a identidade da service account no terminal e consulta `/api/grades?exam=ab1` pela porta configurada. Use esse teste para confirmar rapidamente que os feedbacks da linha real do aluno estao chegando no payload antes do deploy.

## Deploy

No Vercel, nao use `GOOGLE_SERVICE_ACCOUNT_FILE`. Gere a credencial em base64:

```bash
base64 < service-account.local.json | tr -d '\n'
```

Configure:

```env
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<saida>
GOOGLE_SHEET_ID=<id>
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
DOCS_USERNAME=adão
DOCS_PASSWORD=primeiro
```

Deixe `VITE_API_BASE` vazio quando frontend e `/api/*` estiverem no mesmo projeto.
