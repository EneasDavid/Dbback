# Arquitetura

dbBack segue uma organizacao MVC no frontend e servicos por responsabilidade no backend.

## Frontend

- `src/Models`: tipos, cache, normalizacao e compatibilidade de payload.
- `src/Views`: componentes React e estilos.
- `src/Controllers`: fluxo de aplicacao, sessao e chamadas HTTP.

## Backend

- `api`: entrada serverless e roteamento HTTP.
- `pkg/app`: modelos, configuracao, sessao, regras de notas e integracao Google.
- `cmd/dev`: servidor local completo.
- `cmd/comments`: diagnostico de comentarios.

## Fluxo principal

```text
LoginView -> POST /api/login -> AuthController -> LoginIdentity(Base de dados)
AuthController -> warm-up GradesFor -> SheetsClient cache/singleflight
AppController -> GET /api/grades/all -> GradesController
GradesController -> Google Sheets/Drive -> payload JSON -> React Views
```

## Cache e desempenho

- O frontend usa `sessionStorage`, `ETag` e stale-while-revalidate.
- O backend usa cache em memoria por planilha/aba e `singleflight`.
- O login inicia warm-up assincrono das notas.
- A leitura do Google Sheets tenta agrupar abas em batch.
- Comentarios do Drive/XLSX sao enriquecimento; falha neles nao bloqueia leitura de notas.
