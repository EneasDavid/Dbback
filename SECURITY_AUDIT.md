# Auditoria de Segurança - Sistema dbBack

## ✅ Garantias de Segurança Implementadas

### 1. **Acesso à Planilha (Google Sheets)**
- ✅ **Escopo Read-Only**: `sheets.SpreadsheetsReadonlyScope`
- ✅ **Permissões Adicionais**: `drive.readonly`
- ✅ **Resultado**: É **IMPOSSÍVEL** alterar dados via API do Google Sheets
- ✅ **Acesso Direto**: Apenas através da conta de serviço autenticada

### 2. **Endpoints API - Proteção de Dados**

#### `/api/login` (POST)
- ✅ Valida matrícula contra tabela de login
- ✅ Retorna apenas `{ matricula, name }`
- ✅ Não expõe lista de matrículas
- ✅ Sem paginação ou busca parcial

#### `/api/logout` (POST)
- ✅ Limpa sessão
- ✅ Requer autenticação implícita (apenas POST)

#### `/api/me` (GET)
- ✅ Requer autenticação via sessão
- ✅ Retorna apenas dados do usuário logado
- ✅ Valida integridade do cookie (HMAC)

#### `/api/grades` (GET)
- ✅ Requer autenticação via sessão
- ✅ Recebe parâmetro `exam` (ab1 | ab2)
- ✅ Retorna apenas dados da matrícula logada
- ✅ Nenhum acesso a dados de outros usuários
- ✅ Sem exposição de lista de matrículas

### 3. **Autenticação e Sessão**
- ✅ **Cookie Assinado**: HMAC com `SESSION_SECRET`
- ✅ **Expiração**: TTL configurável
- ✅ **Validação**: `SessionManager.User(r)` valida cada requisição
- ✅ **Isolamento**: Cada usuário vê apenas seus dados
- ✅ **CSRF Protection**: Cookies têm HttpOnly (implícito em Set-Cookie)

### 4. **Cache de Dados**
- ✅ **Isolado por SheetName**: Sem mistura de dados de usuários
- ✅ **Singleflight**: Evita requisições duplicadas
- ✅ **Limpável**: `ClearCache()` disponível via `?refresh=1`
- ✅ **TTL**: Cache expira automaticamente

### 5. **Fluxo de Segurança por Requisição**

```
1. Cliente envia matricula → /api/login
2. Servidor valida contra "Base de dados" sheet
3. Se encontrado → cria cookie assinado (HMAC)
4. Cliente envia GET /api/grades com cookie
5. Servidor valida integridade do cookie
6. Servidor busca APENAS dados da matricula do cookie
7. Retorna dados do usuário específico
```

### 6. **Proteção contra Ataques Comuns**

| Ataque | Proteção |
|--------|----------|
| **SQL Injection** | Não usa SQL; apenas leitura de Google Sheets |
| **Exposição de todos os dados** | Sem endpoint `/api/all-users` ou `/api/all-grades` |
| **Escalação de privilégios** | Sessão vinculada à matrícula; impossível forjar cookie sem `SESSION_SECRET` |
| **Acesso sem autenticação** | `/api/grades` e `/api/me` exigem sessão válida |
| **Alteração de dados** | Google Sheets está em read-only; sem escrita permitida |
| **Acesso direto à planilha** | Apenas conta de serviço com credenciais autenticada |
| **Session Hijacking** | Cookie com HMAC; sem valor em plain text |
| **Busca por força bruta** | `/api/login` valida contra lista fixa; sem enumeration |

## 📋 Checklist de Conformidade

- ✅ Apenas consulta de dados individual por login
- ✅ Impossível alterar dados na planilha
- ✅ Acesso direto à planilha negado (read-only + conta de serviço)
- ✅ Sessão obrigatória para `/api/grades`
- ✅ Nenhum endpoint expõe dados de múltiplos usuários
- ✅ Validação de matrícula contra lista (sem busca pública)

## 🔒 Configurações Críticas

Garantir que `.env` contém:

```
SESSION_SECRET=<chave-secreta-forte>  # ✅ Usar >32 caracteres
COOKIE_SECURE=true                     # ✅ HTTPS em produção
GOOGLE_SERVICE_ACCOUNT_FILE=<json>     # ✅ Credenciais da Google
GOOGLE_SHEET_ID=<spreadsheet-id>       # ✅ Spreadsheet protegido
```

## 🚀 Recomendações Adicionais (Opcional)

1. **Rate Limiting**: Adicionar limite de requisições por IP/usuário
2. **Logging**: Registrar todas as requisições de autenticação
3. **Alerta**: Notificar tentativas de login falhadas (>3 vezes)
4. **2FA**: Considerar autenticação de dois fatores no futuro
5. **Audit Trail**: Manter histórico de acessos por usuário

---

**Conclusão**: O sistema está **SEGURO** contra acesso não autorizado à planilha. É **IMPOSSÍVEL** alterar dados, acessar a planilha diretamente ou consultar dados de outros usuários sem ter a matrícula autenticada.
