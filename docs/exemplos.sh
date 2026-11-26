#!/usr/bin/env bash
# Exemplos de uso da API com curl.
# Por padrão usa a API 1 do docker compose (localhost:8081).
# Para usar outra: API=http://localhost:8082 ./docs/exemplos.sh
# Precisa do jq para extrair o token (apt install jq).
set -e
API=${API:-http://localhost:8081}

echo "== cadastro (devolve token)"
curl -s -X POST $API/auth/registrar -H 'Content-Type: application/json' \
  -d '{"email":"rafael@exemplo.com","senha":"123456"}' || true
echo

echo "== login"
TOKEN=$(curl -s -X POST $API/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"rafael@exemplo.com","senha":"123456"}' | jq -r .token)
echo "token: ${TOKEN:0:20}..."

echo "== criar rifa (nasce como rascunho)"
ID=$(curl -s -X POST $API/rifas -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"titulo":"Rifa do gatinho","descricao":"Ajude no tratamento","premio":"Caixa de bombom","preco_centavos":500}' | jq -r .id)
echo "rifa $ID"

echo "== abrir"
curl -s -X POST $API/rifas/$ID/abrir -H "Authorization: Bearer $TOKEN"; echo

echo "== comprar o 42 (sem conta)"
curl -s -X POST $API/rifas/$ID/numeros/42/comprar -H 'Content-Type: application/json' \
  -d '{"nome":"Ana","contato":"65 99999-0000"}'; echo

echo "== comprar o 42 de novo (409)"
curl -s -X POST $API/rifas/$ID/numeros/42/comprar -H 'Content-Type: application/json' \
  -d '{"nome":"Beto","contato":"65 98888-0000"}'; echo

echo "== listar rifas abertas"
curl -s $API/rifas; echo

echo "== fechar e sortear"
curl -s -X POST $API/rifas/$ID/fechar -H "Authorization: Bearer $TOKEN" > /dev/null
curl -s -X POST $API/rifas/$ID/sortear -H "Authorization: Bearer $TOKEN"; echo
