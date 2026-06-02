# Deploy

## Vercel

No Vercel, use credencial Google em base64:

```env
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<saida-base64>
GOOGLE_SHEET_ID=<id>
LOGIN_SHEET_NAME=Base de dados
SESSION_SECRET=<chave-forte>
COOKIE_SECURE=true
```

Mantenha `VITE_API_BASE` vazio quando frontend e `/api/*` estiverem no mesmo projeto.

## Checklist

- Google Sheets API habilitada.
- Google Drive API habilitada se comentarios ricos forem necessarios.
- Planilha compartilhada com o `client_email`.
- `SESSION_SECRET` forte configurado.
- Credenciais de Basic Auth para `/api/docs` configuradas.
- `COOKIE_SECURE=true` em producao.

## Validacao antes de publicar

```bash
npm run quality
go test ./...
```

O workflow `quality-gate` tambem executa lint, build, testes Go, vet, race detector e audit.
