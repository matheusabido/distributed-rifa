// Package config lê a configuração da API a partir de variáveis de ambiente.
// Quem define os valores é o docker-compose (.env + environment:). Veja
// docs/variaveis.md.
package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Peer é outra API: ID (define a hierarquia) + endereço gRPC.
type Peer struct {
	ID   int
	Addr string
}

type API struct {
	NodeID      int
	HTTPAddr    string // onde o Fiber escuta (HTTP + websocket), ex ":8080"
	GRPCAddr    string // onde esta API recebe chamadas gRPC das outras, ex ":9000"
	Peers       []Peer // as OUTRAS APIs
	DBHosts     []string
	DBUser      string
	DBPassword  string
	DBName      string
	databaseURL string // DATABASE_URL, se alguém quiser passar pronta
	JWTSecret   string // o mesmo em todas as APIs
	CORSOrigins string // sites que podem chamar a API pelo navegador ("*" = todos)
}

// TotalAPIs é quantas APIs existem no total (esta + as outras).
func (c API) TotalAPIs() int { return len(c.Peers) + 1 }

// DatabaseURL é a conexão usada para ler e gravar.
//
// Lista TODOS os bancos e pede target_session_attrs=read-write: o pgx tenta
// cada host e fica com o que aceita escrita (o primário). Se o primário
// mudar (db1 caiu e o db2 foi promovido), as conexões novas vão sozinhas
// para o novo primário.
func (c API) DatabaseURL() string {
	if c.databaseURL != "" {
		return c.databaseURL
	}
	return fmt.Sprintf("postgres://%s:%s@%s/%s?target_session_attrs=read-write",
		url.QueryEscape(c.DBUser), url.QueryEscape(c.DBPassword),
		strings.Join(c.DBHosts, ","), c.DBName)
}

// DBHostURL é a conexão com UM banco específico (usada para checar a saúde
// de cada um separadamente e para promover a réplica).
func (c API) DBHostURL(host string) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s",
		url.QueryEscape(c.DBUser), url.QueryEscape(c.DBPassword), host, c.DBName)
}

// Load lê:
//
//	NODE_ID=1
//	HTTP_ADDR=:8080
//	GRPC_ADDR=:9000
//	PEER_APIS=2@api2:9000,3@api3:9000   (ID@endereço das OUTRAS APIs)
//	DB_HOSTS=db1:5432,db2:5432          (primeiro = preferido)
//	DB_USER=postgres  DB_PASSWORD=...  DB_NAME=rifa
//	DATABASE_URL=...                    (opcional; se vier, ignora DB_*)
//	JWT_SECRET=troque-isto
//	CORS_ORIGINS=*
func Load() API {
	return API{
		NodeID:      mustInt("NODE_ID"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		GRPCAddr:    getenv("GRPC_ADDR", ":9000"),
		Peers:       parsePeers(os.Getenv("PEER_APIS")),
		DBHosts:     splitList(getenv("DB_HOSTS", "localhost:5432")),
		DBUser:      getenv("DB_USER", "postgres"),
		DBPassword:  getenv("DB_PASSWORD", "postgres"),
		DBName:      getenv("DB_NAME", "rifa"),
		databaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   getenv("JWT_SECRET", "dev-secret"),
		CORSOrigins: getenv("CORS_ORIGINS", "*"),
	}
}

func parsePeers(s string) []Peer {
	var peers []Peer
	for _, item := range splitList(s) {
		idStr, addr, ok := strings.Cut(item, "@")
		id, err := strconv.Atoi(idStr)
		if !ok || err != nil {
			log.Fatalf("PEER_APIS: %q deve ser ID@endereco, ex 2@api2:9000", item)
		}
		peers = append(peers, Peer{ID: id, Addr: addr})
	}
	return peers
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustInt(k string) int {
	v, err := strconv.Atoi(os.Getenv(k))
	if err != nil {
		log.Fatalf("variável %s obrigatória e numérica", k)
	}
	return v
}
