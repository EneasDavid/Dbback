# API e Payload

## Endpoints

| Metodo | Caminho | Auth | Descricao |
| --- | --- | --- | --- |
| `GET` | `/api`, `/api/index`, `/api/index.go` | Basic Auth docs | Alias da documentacao da API. |
| `GET` | `/api/docs` | Basic Auth docs | Documentacao HTML. |
| `GET` | `/api/docs?format=json` | Basic Auth docs | Documentacao JSON. |
| `POST` | `/api/login` | Nao | Valida matricula e cria cookie de sessao. |
| `POST` | `/api/logout` | Nao | Limpa a sessao. |
| `GET` | `/api/me` | Cookie opcional | Retorna usuario da sessao ou `null`. |
| `GET` | `/api/grades?exam=<avaliacao>` | Cookie | Retorna uma avaliacao. |
| `GET` | `/api/grades/<avaliacao>` | Cookie | Alias path-based da avaliacao. |
| `GET` | `/api/grades/all` | Cookie | Retorna todas as avaliacoes disponiveis. |

## Cache HTTP

- Sessao e docs: `Cache-Control: no-store`.
- Notas: cache privado por usuario, `ETag`, `Vary: Cookie, Accept-Encoding`.
- `refresh=1` limpa cache em memoria antes de ler dados.

## Payload de notas

O backend devolve tabelas render-ready. Cada tabela pode conter cards de atividade, cards de resumo e detalhes de criterios.

Campos principais:

- `exam`: chave da avaliacao.
- `matricula`: matricula do aluno.
- `schemaStatus`: origem conhecida, como `legacy` ou `v2`.
- `tables`: lista de tabelas com `kind`, `status`, `cards` e `details`.

Os criterios incluem label, valor, nota maxima, score exibido, ratio para barra de progresso e comentario quando existir.
