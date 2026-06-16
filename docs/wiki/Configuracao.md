# Configuracao

## Variaveis principais

```env
GOOGLE_SHEET_ID=...
GOOGLE_SHEET_IDS=...
GOOGLE_SHEET_LEGACY_IDS=...
GOOGLE_SHEET_V2_IDS=...
SHEETS_RUNTIME_VERSION=auto
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<json-em-base64>
TURNSTILE_SECRET_KEY=<secret-key-do-turnstile>
VITE_TURNSTILE_SITE_KEY=<site-key-publica-do-turnstile>
```

Use `GOOGLE_SHEET_LEGACY_IDS` e `GOOGLE_SHEET_V2_IDS` quando planilhas antigas e novas existirem ao mesmo tempo.

O "nao sou um robo" do login e obrigatorio. Configure Cloudflare Turnstile com as duas variaveis. A chave `VITE_TURNSTILE_SITE_KEY` aparece no navegador; `TURNSTILE_SECRET_KEY` fica privada no backend e valida o token antes de qualquer consulta a planilha. Sem uma das duas chaves, o login fica bloqueado por configuracao incompleta.

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
