# GitHub Project: dbBack

## Objetivo
Organizar a entrega de melhorias de UX e experiência de lançamento estável para o dbBack.

## Visão
- Melhorar a navegação do usuário no dropdown de atividades.
- Garantir que, ao abrir detalhes, o foco visual fique em `Critérios avaliados`.
- Expor uma flag de versão estável no frontend.
- Documentar o projeto e o fluxo de entregas no repositório.

## Itens do projeto

### 1. UX de ativação de dropdown
- [x] Ajustar o scroll automático ao abrir detalhes de atividade.
- [ ] Garantir comportamento suave e respeitar `prefers-reduced-motion`.

### 2. Flag de versão estável
- [x] Criar `appVersion.stable` para sinalizar lançamento estável.
- [x] Expor `data-stable` no elemento raiz do documento.
- [x] Mostrar badge `v2` no topo da aplicação.

### 3. Estrutura do GitHub Project
- [x] Adicionar este arquivo de projeto no repositório.
- [ ] Criar issues relacionadas ao sprint de usabilidade e estabilidade.

## Cronograma curto
1. Validar scroll no painel de detalhes.
2. Confirmar badge de versão visível para usuários.
3. Abrir issues para continuação das melhorias.

## Observações
- Esta aplicação já possui uma versão de runtime `v2` no modelo de versão com `v2_stable: true`.
- O projeto GitHub aqui é um ponto central para coordenar as próximas entregas, mesmo que a gestão de cards seja feita fora do repositório.
