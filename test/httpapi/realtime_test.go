package httpapi_test

// Teste do tempo real com DUAS APIs de verdade (portas HTTP e gRPC reais):
//
//	navegador A ──ws──► API 1 ◄──gRPC──► API 2 ◄──ws── navegador B
//
// Alguém compra o número 42 pela API 1. Os DOIS navegadores precisam receber
// o evento "ticket_bought": A direto pelo hub da API 1, e B porque a API 1
// avisou a API 2 por gRPC.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	fws "github.com/fasthttp/websocket"
	"google.golang.org/grpc"

	"rifa/gen/rifapb"
	"rifa/internal/auth"
	"rifa/internal/httpapi"
	"rifa/internal/realtime"
	"rifa/internal/store"
)

type apiNode struct {
	httpURL string
	wsURL   string
	hub     *realtime.Hub
}

func TestTempoRealEntreAPIs(t *testing.T) {
	setup(t) // limpa o banco (e pula se não houver TEST_DATABASE_URL)
	st, err := store.Connect(context.Background(), os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)

	// Reserva as portas gRPC antes, porque cada API precisa saber o
	// endereço da outra.
	grpc1, grpc2 := listen(t), listen(t)
	api1 := startAPI(t, st, 1, grpc1, grpc2.Addr().String())
	api2 := startAPI(t, st, 2, grpc2, grpc1.Addr().String())

	// Dois "navegadores", um em cada API.
	navA := dialWS(t, api1.wsURL)
	navB := dialWS(t, api2.wsURL)
	waitFor(t, func() bool { return api1.hub.Count() == 1 && api2.hub.Count() == 1 })

	// Prepara uma rifa aberta (pela API 1).
	token := httpPost(t, api1.httpURL+"/auth/registrar", "", `{"email":"d@d.com","senha":"123456"}`)["token"].(string)
	id := int(httpPost(t, api1.httpURL+"/rifas", token, `{"titulo":"t","premio":"p"}`)["id"].(float64))
	httpPost(t, api1.httpURL+fmt.Sprintf("/rifas/%d/abrir", id), token, ``)

	// Compra pela API 1.
	httpPost(t, api1.httpURL+fmt.Sprintf("/rifas/%d/numeros/42/comprar", id), "", `{"nome":"Ana","contato":"x"}`)

	for nome, nav := range map[string]*fws.Conn{"A (API 1)": navA, "B (API 2)": navB} {
		msg := readUntil(t, nav, "ticket_bought")
		dados := msg["dados"].(map[string]any)
		if dados["numero"].(float64) != 42 || dados["nome"] != "Ana" {
			t.Fatalf("navegador %s recebeu dados errados: %v", nome, msg)
		}
		if _, vazou := dados["contato"]; vazou {
			t.Fatalf("contato vazou no evento para o navegador %s", nome)
		}
		if msg["origem"].(float64) != 1 {
			t.Fatalf("navegador %s: origem deveria ser a API 1: %v", nome, msg)
		}
	}
}

// --- ajudantes ---

func listen(t *testing.T) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0") // porta 0 = o sistema escolhe uma livre
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func startAPI(t *testing.T, st *store.Store, id int, grpcLn net.Listener, peer string) apiNode {
	hub := realtime.NewHub()
	cluster, err := realtime.NewCluster(id, hub, []string{peer})
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	rifapb.RegisterBroadcastServer(gs, cluster)
	go gs.Serve(grpcLn)
	t.Cleanup(gs.Stop)

	app := httpapi.New(st, auth.New("segredo"), cluster, id, hub).App()
	httpLn := listen(t)
	go app.Listener(httpLn)
	t.Cleanup(func() { _ = app.Shutdown() })

	addr := httpLn.Addr().String()
	return apiNode{httpURL: "http://" + addr, wsURL: "ws://" + addr + "/ws", hub: hub}
}

func dialWS(t *testing.T, url string) *fws.Conn {
	conn, _, err := fws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("websocket %s: %v", url, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readUntil lê mensagens do websocket até achar uma do tipo pedido.
func readUntil(t *testing.T, conn *fws.Conn, tipo string) map[string]any {
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("esperando %q: %v", tipo, err)
		}
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(err)
		}
		if msg["tipo"] == tipo {
			return msg
		}
	}
}

func httpPost(t *testing.T, url, token, body string) map[string]any {
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s: %d %v", url, resp.StatusCode, out)
	}
	return out
}

func waitFor(t *testing.T, cond func() bool) {
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("tempo esgotado")
}
