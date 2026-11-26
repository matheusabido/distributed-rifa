#!/usr/bin/env bash
# Roteiro automático de falhas. Rode com o sistema de pé (docker compose up -d),
# na pasta do projeto:   ./tools/teste-falhas.sh
#
# 1. derruba a API 1 (mestre)     -> API 2 vira mestre, o site continua
# 2. derruba o db1 (primário)     -> a mestre promove o db2, compras voltam
# 3. religa o db1                 -> volta como RÉPLICA do db2
# 4. religa a API 1               -> volta a ser a mestre
# Precisa de curl e jq.
set -e
SITE=${SITE:-http://localhost:3000/api}
J='Content-Type: application/json'

passo() { echo; echo "=== $(date -u +%T) UTC: $*"; }
cluster() {  # o que a API $1 (porta 808$1) acha do sistema
  curl -s "http://localhost:808$1/cluster" | jq -c '{eu, mestre, apis: [.apis[] | "\(.nome)=\(.estado)"], bancos: [.bancos[] | "\(.nome)=\(.estado)"]}'
}
comprar() {
  curl -s -o /dev/null -w '%{http_code}' -X POST "$SITE/rifas/$ID/numeros/$1/comprar" -H "$J" -d '{"nome":"Teste","contato":"x"}'
}

passo "preparando uma rifa aberta"
curl -s -X POST $SITE/auth/registrar -H "$J" -d '{"email":"falhas@teste.com","senha":"123456"}' > /dev/null
TOKEN=$(curl -s -X POST $SITE/auth/login -H "$J" -d '{"email":"falhas@teste.com","senha":"123456"}' | jq -r .token)
ID=$(curl -s -X POST $SITE/rifas -H "Authorization: Bearer $TOKEN" -H "$J" -d '{"titulo":"Teste de falhas","premio":"bombom"}' | jq .id)
curl -s -X POST $SITE/rifas/$ID/abrir -H "Authorization: Bearer $TOKEN" > /dev/null
echo "rifa $ID aberta"; cluster 2

passo "1) derrubando a API 1 (mestre)"
docker compose stop api1
sleep 8
cluster 2
echo "compras pelo site: $(comprar 1) $(comprar 2) $(comprar 3)   (esperado: 201)"

passo "2) derrubando o db1 (banco primário)"
docker compose stop db1
for n in $(seq 10 21); do echo "$(date -u +%T) compra $n: $(comprar $n)"; sleep 1; done
echo "(503 = banco indisponível durante a troca; depois volta 201)"
cluster 2

passo "3) religando o db1 (deve voltar como réplica)"
docker compose start db1
sleep 15
docker compose logs db1 --since 20s | grep '\[db1\]' || true
cluster 2

passo "4) religando a API 1 (deve voltar a ser mestre)"
docker compose start api1
sleep 9
cluster 2
cluster 3

passo "dados iguais nos dois bancos?"
for db in db1 db2; do
  echo "$db: $(docker compose exec -T $db psql -U postgres -d rifa -tAc "select count(*) from tickets where raffle_id = $ID") números vendidos"
done
