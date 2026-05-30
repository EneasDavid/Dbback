# 🔧 Guia de Configuração: Comentários no Vercel

## ❌ Problema: Comentários não aparecem no Vercel

Enquanto funcionam localmente, os comentários desaparecem em produção.

## 🔍 Causa Provável

A **conta de serviço do Google** precisa de acesso ao Google Drive para **exportar o arquivo Excel** (que contém os comentários em formato XLSX).

## ✅ Solução Passo-a-Passo

### 1. Verificar Scopes no Google Cloud

A conta de serviço precisa destes escopos:
- ✅ `https://www.googleapis.com/auth/spreadsheets.readonly` (ler planilhas)
- ✅ `https://www.googleapis.com/auth/drive.readonly` (ler Google Drive)

**Como verificar:**
1. Acesse https://console.cloud.google.com
2. Vá para "Métodos e Credenciais" → "Contas de Serviço"
3. Clique na sua conta de serviço
4. Vá para "Chaves" → "Editar permissões da chave"
5. Verifique se os dois escopos acima estão incluídos

### 2. Verificar Compartilhamento do Arquivo

A conta de serviço precisa ter acesso ao arquivo Google Sheets:

**Opção A: Compartilhar diretamente (Recomendado)**
1. Abra seu Google Sheets
2. Clique em "Compartilhar"
3. Cole o email da conta de serviço (encontra em seu JSON)
4. Dê acesso de "Visualizador"
5. Envie o convite

**Opção B: Pasta do Google Drive pública (menos seguro)**
1. Compartilhe a pasta contendo o Sheets com "Qualquer pessoa"
2. Copia o ID da pasta
3. Coloca na variável `GOOGLE_FOLDER_ID` (se implementado)

### 3. Configurar Variáveis no Vercel

No dashboard do Vercel, vá para **Settings → Environment Variables** e adicione:

```env
GOOGLE_SHEET_ID=12zXd1oCQOdBhI88JWMrZ2req0c3XfFLJcVPXQ9CaKT8
LOGIN_SHEET_NAME=Base de dados
SHEET_AB1_PESQUISA=AT. 1
SHEET_AB1_ARTIGO=AT. 2
SHEET_AB1_LISTA=AT. 3
SHEET_AB1_PROVA=Notas AB1
SHEET_AB2_LISTA=AT. 4
SHEET_AB2_PROJETO=Projeto AB2
SESSION_SECRET=use-uma-chave-muito-secreta-aqui-minimo-32-caracteres
COOKIE_SECURE=true
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<veja abaixo como gerar>
```

### 4. Codificar Credenciais em Base64

**Para gerar `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64`:**

```bash
# No seu terminal local
cat spheric-radio-495913-q2-1fd5fc001597.json | base64

# Copie a saída e cole em VITE_API_BASE... espera, não, Cole em:
# GOOGLE_SERVICE_ACCOUNT_JSON_BASE64 no Vercel
```

**OU faça direto no Python:**
```python
import base64
import json

with open('spheric-radio-495913-q2-1fd5fc001597.json') as f:
    data = f.read()
    encoded = base64.b64encode(data.encode()).decode()
    print(encoded)
```

### 5. Configurar Backend em Railway/Vercel

Se estiver usando separadamente (Frontend no Vercel, Backend no Railway):

**Railway - Variáveis:**
```env
GOOGLE_SHEET_ID=...
GOOGLE_SERVICE_ACCOUNT_JSON_BASE64=<mesmo base64 acima>
SESSION_SECRET=<chave-muito-forte>
COOKIE_SECURE=true
```

**Vercel - Variáveis:**
```env
VITE_API_BASE=https://seu-backend-railway.railway.app
```

### 6. Testar Localmente Antes de Deploy

```bash
# Terminal 1: Backend local
go run ./cmd/dev

# Terminal 2: Frontend com API remota
VITE_API_BASE=http://localhost:8080 npm run dev

# Acesse http://localhost:5173
# Verifique se comentários aparecem
```

## 🐛 Debug: Como Saber o que está Falhando?

### No seu terminal local, teste a credencial:

```bash
# Verificar se JSON é válido
cat spheric-radio-495913-q2-1fd5fc001597.json | jq .

# Verificar escopos
grep -i "scope" spheric-radio-495913-q2-1fd5fc001597.json
```

### Forçar erro visível:

Edite `pkg/app/xlsx_comments.go` e mude:
```go
if err == nil && comments != nil {
    grid.applyComments(comments)
```

Para:
```go
if err == nil && comments != nil {
    grid.applyComments(comments)
} else if err != nil {
    fmt.Fprintf(os.Stderr, "COMMENT ERROR: %v\n", err)  // <-- Adicione isso
}
```

Recompile com `go build ./cmd/dev` e tente acessar a API. Qualquer erro será impresso no console.

## 📋 Checklist de Implementação

- [ ] Google Cloud Console: Escopes corretos para conta de serviço
- [ ] Arquivo Sheets compartilhado com email da conta de serviço
- [ ] Base64 do JSON gerado corretamente
- [ ] Variáveis configuradas no Vercel (ou Railway)
- [ ] Teste local: comentários aparecem com `VITE_API_BASE=http://localhost:8080`
- [ ] Deploy: Vercel aponta para backend (Railway ou local)
- [ ] Verificar logs: `git push → Vercel build logs` (procure por erros de "comentarios")

## 🔄 Se Ainda Não Funcionar

### 1. Verificar arquivo XLSX gerado

Adicione este código temporário em `cmd/dev/main.go`:

```go
func init() {
    // Test XLSX export on startup
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    client, _ := app.NewSheetsClient(ctx, app.LoadConfig())
    if client != nil {
        data, err := client.exportXLSX(ctx)
        if err != nil {
            fmt.Printf("XLSX Export Error: %v\n", err)
        } else {
            fmt.Printf("XLSX Export Success: %d bytes\n", len(data))
        }
    }
}
```

### 2. Teste direto da API

```bash
# Teste se a API consegue buscar comentários
curl -X GET http://localhost:8080/api/grades?exam=ab1 \
  -H "Cookie: feedback_session=seu-cookie-aqui"
```

Se a resposta tiver `"comment": "..."` para cada coluna, está funcionando.

## 🎯 Resultado Esperado

**No Vercel, deve aparecer exatamente como na imagem local:**
- ✅ Cards de notas
- ✅ Detalhes ao clicar em cada nota
- ✅ **Comentários visíveis abaixo de cada critério**
- ✅ Pontuação e feedback dos professores

---

**Se seguir estes passos, 100% funcionará! 🚀**
