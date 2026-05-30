# Guia de Deployment - Backend Go + Frontend Vercel

## 📋 Por que é necessário?

- **Localmente**: `npm run dev:full` roda React + servidor Go no mesmo processo
- **Vercel**: Só roda código Node.js/JavaScript estático. Go não pode rodar em Vercel
- **Solução**: Separar backend (Railway) e frontend (Vercel)

---

## 🚀 OPÇÃO 1: Railway (Recomendado - Gratuito)

### 1. Criar projeto Railway

1. Acesse https://railway.app
2. Clique em "New Project" → "Deploy from GitHub"
3. Selecione o repositório `EneasDavid/Dbback`
4. Clique em "Deploy Now"

### 2. Configurar variáveis de ambiente

No dashboard do Railway, abra seu projeto e vá para "Variables":

```env
GOOGLE_SHEET_ID=12zXd1oCQOdBhI88JWMrZ2req0c3XfFLJcVPXQ9CaKT8
LOGIN_SHEET_NAME=Base de dados
SHEET_AB1_PESQUISA=AT. 1
SHEET_AB1_ARTIGO=AT. 2
SHEET_AB1_LISTA=AT. 3
SHEET_AB1_PROVA=Notas AB1
SHEET_AB2_LISTA=AT. 4
SHEET_AB2_PROJETO=Projeto AB2
SESSION_SECRET=use-uma-chave-secreta-forte-aqui-min-32-chars
COOKIE_SECURE=true
GOOGLE_SERVICE_ACCOUNT_FILE=spheric-radio-495913-q2-1fd5fc001597.json
```

Se Railway não conseguir ler o arquivo `.json` direto, use:

```env
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=eyJ0eXBlIjoic2VydmljZV9hY2NvdW50Ii8uLi4=
```

(Encode o JSON em base64: `cat spheric-radio-495913-q2-1fd5fc001597.json | base64`)

### 3. Configurar build e start

No Railway, crie um arquivo `Procfile` na raiz:

```procfile
web: go run ./cmd/dev
```

### 4. Copiar arquivo de credenciais

Você precisa do arquivo JSON no Railway. Opções:

**A) Via Railway CLI:**
```bash
railway run "cat > spheric-radio-495913-q2-1fd5fc001597.json <<'EOF'
{coloque-o-conteudo-do-json}
EOF"
```

**B) Via secrets base64:**
Encode o JSON em base64 e use a variável `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64` no código Go.

### 5. Obter URL do backend

Após deploy bem-sucedido, Railway gera uma URL como:
```
https://dbback-production-xxxx.railway.app
```

---

## 🔗 Conectar Vercel ao Railway Backend

### 1. Adicionar variável no Vercel

1. Acesse https://vercel.com/dashboard
2. Vá para seu projeto "feedback-notas"
3. Clique em "Settings" → "Environment Variables"
4. Adicione:

```
VITE_API_BASE=https://dbback-production-xxxx.railway.app
```

(Substitua pela URL real do Railway)

### 2. Testar localmente

```bash
# Terminal 1: Backend
go run ./cmd/dev

# Terminal 2: Frontend (em outro terminal)
VITE_API_BASE=http://localhost:8080 npm run dev
```

### 3. Deploy

```bash
git add .
git commit -m "refactor: separate backend and frontend deployment"
git push origin main
```

Vercel fará deploy automaticamente. Railway também fará rebuild se houver mudança no repositório.

---

## 🔓 CORS (Importante!)

Se o frontend e backend estão em domínios diferentes, você precisa habilitar CORS.

No seu `cmd/dev/main.go` ou onde o servidor é iniciado, adicione middleware CORS:

```go
package main

import (
	"log"
	"net/http"
)

func init() {
	http.HandleFunc("/", corsMiddleware(handler.Handler))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next(w, r)
	}
}
```

**Mas cuidado**: Permitir `*` é inseguro em produção. Use:

```go
allowedOrigins := []string{
	"https://seu-frontend-vercel.vercel.app",
	"http://localhost:5173",
}
```

---

## 🔐 Segurança com CORS e Cookies

Como estão em domínios diferentes, cookies podem não funcionar automaticamente. Solução:

No `src/api.ts`, já está configurado:
```typescript
credentials: 'include'  // Envia cookies cross-domain
```

No backend, adicione:
```go
w.Header().Set("Access-Control-Allow-Credentials", "true")
```

---

## 📝 OPÇÃO 2: Render (Alternativa ao Railway)

1. Acesse https://render.com
2. Clique em "New +" → "Web Service"
3. Conecte GitHub
4. Runtime: "Go"
5. Build Command: `go build -o app ./cmd/dev`
6. Start Command: `./app`

Render é similar a Railway, apenas interface diferente.

---

## 💾 OPÇÃO 3: Cloud Run (Google Cloud - Mais caro)

Se preferir ficar no Google Cloud:

1. Instale Cloud SDK: https://cloud.google.com/sdk/docs/install
2. Crie um `Dockerfile`:

```dockerfile
FROM golang:1.23-alpine
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/dev
EXPOSE 8080
CMD ["./server"]
```

3. Deploy:
```bash
gcloud run deploy dbback --source . --platform managed --region us-central1
```

---

## ✅ Checklist Final

- [ ] Backend deployado em Railway/Render (URL anotada)
- [ ] Variáveis de ambiente configuradas no Railway
- [ ] Arquivo JSON de credenciais do Google está no Railway
- [ ] `VITE_API_BASE` configurado no Vercel
- [ ] Teste local: `VITE_API_BASE=http://localhost:8080 npm run dev`
- [ ] Teste em produção: Vercel chamando Railway
- [ ] CORS habilitado se backend e frontend estão em domínios diferentes

---

## 🐛 Debug

**Se receberAssistant CORS error:**
```
Access to XMLHttpRequest at 'https://railway...' from origin 'https://vercel...'
has been blocked by CORS policy
```

Adicione CORS middleware no backend (veja acima).

**Se cookies não funcionam:**
```
Set-Cookie não aparece no Response Headers
```

Você precisa de:
1. `Access-Control-Allow-Credentials: true` no backend
2. `credentials: 'include'` no frontend (já está em `src/api.ts`)

**Se backend retorna 401 em produção:**
Verifique se `SESSION_SECRET` é o mesmo em local e Railway.

---

**Uma vez feito isso, seu app funcionará no Vercel! ✅**
