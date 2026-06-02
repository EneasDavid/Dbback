# dbBack Wiki

dbBack e uma aplicacao Go + React para consulta autenticada de notas e feedbacks a partir de Google Sheets. A wiki organiza o que precisa ficar facil de achar no GitHub: instalacao, configuracao, arquitetura, API, deploy e pacote.

## Links rapidos

- [Instalacao](Instalacao)
- [Configuracao](Configuracao)
- [Arquitetura](Arquitetura)
- [API e Payload](API-e-Payload)
- [Deploy](Deploy)
- [Pacote GitHub](Pacote-GitHub)

## Contrato do projeto

- O aluno acessa com matricula.
- O backend valida a identidade na aba `Base de dados`.
- A sessao fica presa a planilha correta sem expor o `spreadsheetId` ao frontend.
- As notas sao lidas no Google Sheets e enriquecidas com comentarios quando disponiveis.
- O frontend renderiza o payload normalizado sem recalcular regras sensiveis de nota.

## Como esta wiki vai para o GitHub

Os arquivos fonte ficam em `docs/wiki`. O workflow `sync-wiki` copia esse conteudo para o repositorio wiki do GitHub (`Dbback.wiki.git`) quando houver push em `main` ou execucao manual.

Se o push falhar, confirme em GitHub > Settings > Features que a opcao Wiki esta habilitada.
