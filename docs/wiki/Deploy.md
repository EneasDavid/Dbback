# Deploy

## Vercel

No Vercel, use credencial Google em base64:

```env
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<saida-base64>
GOOGLE_SHEET_ID=<id>
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
TURNSTILE_SECRET_KEY=<secret-key-do-turnstile>
VITE_TURNSTILE_SITE_KEY=<site-key-publica-do-turnstile>
```

Mantenha `VITE_API_BASE` vazio quando frontend e `/api/*` estiverem no mesmo projeto.

## Checklist

- Google Sheets API habilitada.
- Google Drive API habilitada se comentarios ricos forem necessarios.
- Planilha compartilhada com o `client_email`.
- `SESSION_SECRET` forte configurado.
- Cloudflare Turnstile obrigatorio configurado com site key publica e secret key privada.
- Credenciais de Basic Auth para `/api/docs` configuradas.
- `COOKIE_SECURE=true` em producao.

## Validacao antes de publicar

```bash
npm run quality
go test ./...
```

O workflow `quality-gate` tambem executa lint, build, testes Go, vet, race detector e audit.
