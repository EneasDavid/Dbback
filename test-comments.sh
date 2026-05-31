#!/bin/bash

# Test script para verificar se comentários estão funcionando
# Use: bash test-comments.sh

set -e
PORT="${PORT:-3000}"
API_BASE="${API_BASE:-http://127.0.0.1:$PORT}"
CREDENTIAL_FILE="${GOOGLE_SERVICE_ACCOUNT_FILE:-./service-account.local.json}"

echo "🔍 Testando configuração de comentários..."
echo ""

# Cores
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. Verificar arquivo de credenciais
echo -n "1️⃣  Verificando credenciais do Google... "
if [ -f "$CREDENTIAL_FILE" ]; then
    echo -e "${GREEN}✓${NC} Arquivo encontrado"

    # Verificar se é JSON válido
    if jq . "$CREDENTIAL_FILE" > /dev/null 2>&1; then
        echo -e "   ${GREEN}✓ JSON válido${NC}"
    else
        echo -e "   ${RED}✗ JSON inválido${NC}"
        exit 1
    fi

    # Verificar se tem os campos esperados
    if jq -e '.type == "service_account"' "$CREDENTIAL_FILE" > /dev/null 2>&1; then
        echo -e "   ${GREEN}✓ Tipo correto (service_account)${NC}"
    else
        echo -e "   ${RED}✗ Tipo incorreto${NC}"
        exit 1
    fi

    EMAIL_DOMAIN=$(jq -r '.client_email // "" | split("@") | .[1] // "dominio-nao-informado"' "$CREDENTIAL_FILE")
    echo -e "   ${GREEN}✓ Conta de serviço detectada${NC} (${YELLOW}${EMAIL_DOMAIN}${NC})"
    echo ""
else
    echo -e "${RED}✗${NC} Arquivo não encontrado"
    echo "   Defina GOOGLE_SERVICE_ACCOUNT_FILE ou use ./service-account.local.json"
    exit 1
fi

# 2. Verificar .env
echo -n "2️⃣  Verificando variáveis de ambiente... "
if [ ! -f ".env" ]; then
    echo -e "${RED}✗${NC} Arquivo .env não encontrado"
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# 3. Verificar se o servidor Go consegue compilar
echo -n "3️⃣  Compilando backend Go... "
if go build -o /tmp/dbback-test ./cmd/dev 2>&1 | grep -i error; then
    echo -e "${RED}✗${NC} Erro na compilação"
    exit 1
else
    echo -e "${GREEN}✓${NC}"
fi

# 4. Testar credenciais com curl (requer servidor rodando)
echo ""
echo -n "4️⃣  Iniciando servidor... "
GOOGLE_SERVICE_ACCOUNT_FILE="$CREDENTIAL_FILE" PORT="$PORT" /tmp/dbback-test &
SERVER_PID=$!
sleep 2

if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${RED}✗${NC} Servidor não iniciou"
    exit 1
fi
echo -e "${GREEN}✓${NC} Servidor rodando (PID: $SERVER_PID)"

# 5. Testar login
echo -n "5️⃣  Testando login (matrícula: 000001)... "
LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE/api/login" \
    -H "Content-Type: application/json" \
    -d '{"matricula":"000001"}' 2>/dev/null)

if echo "$LOGIN_RESPONSE" | jq -e '.matricula' > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
    echo "   Usuário: $(echo $LOGIN_RESPONSE | jq -r '.name')"
    
    # Extrair cookie da resposta
    COOKIE=$(curl -s -i -X POST "$API_BASE/api/login" \
        -H "Content-Type: application/json" \
        -d '{"matricula":"000001"}' 2>/dev/null | grep "Set-Cookie" | cut -d' ' -f2 | cut -d';' -f1)
    
    if [ -n "$COOKIE" ]; then
        echo -e "   ${GREEN}✓ Cookie obtido${NC}"
    fi
else
    echo -e "${RED}✗${NC} Login falhou"
    echo "   Resposta: $LOGIN_RESPONSE"
    kill $SERVER_PID 2>/dev/null
    exit 1
fi

# 6. Testar busca de notas com comentários
echo -n "6️⃣  Buscando notas (exam=ab1)... "
GRADES_RESPONSE=$(curl -s "$API_BASE/api/grades?exam=ab1" \
    -H "Cookie: $COOKIE" 2>/dev/null)

if echo "$GRADES_RESPONSE" | jq -e '.tables[0].cards[0]' > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
    
    # Verificar se há comentários
    HAS_COMMENTS=$(echo "$GRADES_RESPONSE" | jq '[(.tables[].cards[]?, .tables[].cards[]?.details[]?) | select(.comment != null and .comment != "")] | length' 2>/dev/null || echo "0")
    
    if [ "$HAS_COMMENTS" -gt 0 ]; then
        echo -e "   ${GREEN}✓ Comentários encontrados: $HAS_COMMENTS${NC}"
        echo ""
        echo "📋 Exemplo de comentário:"
        echo "$GRADES_RESPONSE" | jq '[(.tables[].cards[]?, .tables[].cards[]?.details[]?) | select(.comment != null and .comment != "")][0]' 2>/dev/null | head -10
    else
        echo -e "   ${YELLOW}⚠ Nenhum comentário encontrado${NC}"
        echo "   Possíveis causas:"
        echo "   1. A planilha não tem notas de célula nos critérios"
        echo "   2. O arquivo não está compartilhado com a conta de serviço"
        echo "   3. A nota está em comentário/discussão do Drive, não em nota de célula"
    fi
else
    echo -e "${RED}✗${NC} Falha ao buscar notas"
    echo "   Resposta: $GRADES_RESPONSE"
    kill $SERVER_PID 2>/dev/null
    exit 1
fi

# Limpar
echo ""
echo -n "Limpando... "
kill $SERVER_PID 2>/dev/null
echo -e "${GREEN}✓${NC}"

echo ""
echo -e "${GREEN}✅ Teste completo!${NC}"
echo ""
echo "Próximas ações:"
echo "1. Se os comentários foram encontrados: tudo está OK! Faça deploy no Vercel."
echo "2. Se NENHUM comentário foi encontrado:"
echo "   a) Verifique se a conta de serviço está com acesso de leitor ao Google Sheets"
echo "   b) Compartilhe o arquivo Sheets com o email da conta (mostrado acima)"
echo "   c) Rode este teste novamente"
