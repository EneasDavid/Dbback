# Configuracao

## Variaveis principais

```env
GOOGLE_SHEET_V2_IDS=...
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<json-em-base64>
TURNSTILE_SECRET_KEY=<secret-key-do-turnstile>
VITE_TURNSTILE_SITE_KEY=<site-key-publica-do-turnstile>
```

`GOOGLE_SHEET_V2_IDS` e a configuracao preferida. O backend ainda aceita `GOOGLE_SHEET_ID` e `GOOGLE_SHEET_IDS` por compatibilidade, nessa ordem depois dos IDs v2. As listas aceitam virgula, ponto e virgula ou quebra de linha e IDs repetidos sao removidos.

O "nao sou um robo" do login e ativado quando as duas variaveis do Cloudflare Turnstile estao configuradas. A chave `VITE_TURNSTILE_SITE_KEY` aparece no navegador; `TURNSTILE_SECRET_KEY` fica privada no backend e valida o token antes de qualquer consulta a planilha. Sem essas chaves, o app nao renderiza o widget e o backend pula a validacao para facilitar desenvolvimento local.

Para usar chaves reais no `localhost`, adicione `localhost` e `127.0.0.1` ao widget no painel da Cloudflare. Para teste local sem widget real, substitua as duas variaveis pelas chaves dummy oficiais da Cloudflare.

## Credenciais Google

Credenciais aceitas:

- `GOOGLE_SERVICE_ACCOUNT_JSON`
- `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`
- `GOOGLE_SERVICE_ACCOUNT_FILE` apenas em desenvolvimento local

Nao envie JSON de service account para o GitHub. Gere base64 para deploy:

```bash
base64 < service-account.local.json | tr -d '\n'
```

Compartilhe a planilha com o `client_email` da service account. Para comentarios ricos, habilite Google Sheets API e Google Drive API no mesmo projeto Google Cloud.

## Docs da API

A rota `/api/docs` usa Basic Auth separado. Configure usuario e senha apenas no ambiente da aplicacao.
