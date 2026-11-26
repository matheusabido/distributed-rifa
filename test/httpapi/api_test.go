package httpapi_test

// Testes de ponta a ponta: sobem o app Fiber e falam com um Postgres de
// verdade. Para rodar:
//
//	TEST_DATABASE_URL=postgres://rifa:rifa@localhost:5432/rifa go test ./...
//
// ATENÇÃO: os testes apagam as tabelas do banco indicado. Use um banco de
// teste, nunca o de produção.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"rifa/internal/auth"
	"rifa/internal/httpapi"
	"rifa/internal/store"
)

func setup(t *testing.T) *fiber.App {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("defina TEST_DATABASE_URL para rodar os testes com banco")
	}
	ctx := context.Background()

	// limpa as tabelas
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE tickets, raffles, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	st, err := store.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return httpapi.New(st, auth.New("segredo-de-teste"), httpapi.LogNotifier{}, 1, nil).App()
}

// call faz uma requisição e devolve status + corpo decodificado.
func call(t *testing.T, app *fiber.App, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 && raw[0] == '{' {
		_ = json.Unmarshal(raw, &out)
	} else {
		out["lista"] = string(raw)
	}
	return resp.StatusCode, out
}

func registerUser(t *testing.T, app *fiber.App, email string) string {
	t.Helper()
	code, body := call(t, app, "POST", "/auth/registrar", "", map[string]any{"email": email, "senha": "123456"})
	if code != http.StatusCreated {
		t.Fatalf("registrar: %d %v", code, body)
	}
	return body["token"].(string)
}

func TestFluxoCompleto(t *testing.T) {
	app := setup(t)
	dono := registerUser(t, app, "dono@rifa.com")

	// login funciona e senha errada não
	code, _ := call(t, app, "POST", "/auth/login", "", map[string]any{"email": "dono@rifa.com", "senha": "123456"})
	expect(t, "login", code, 200)
	code, _ = call(t, app, "POST", "/auth/login", "", map[string]any{"email": "dono@rifa.com", "senha": "errada"})
	expect(t, "login senha errada", code, 401)

	// criar sem token -> 401
	code, _ = call(t, app, "POST", "/rifas", "", map[string]any{"titulo": "x", "premio": "y"})
	expect(t, "criar sem token", code, 401)

	// criar
	code, r := call(t, app, "POST", "/rifas", dono, map[string]any{
		"titulo": "Rifa do gatinho", "premio": "Caixa de bombom", "preco_centavos": 500,
	})
	expect(t, "criar", code, 201)
	id := int(r["id"].(float64))
	if r["status"] != "draft" || r["total_numeros"].(float64) != 200 {
		t.Fatalf("rifa criada errada: %v", r)
	}

	// datas sempre em UTC (mesmo se o banco estiver configurado em outro fuso)
	criada, err := time.Parse(time.RFC3339Nano, r["criada_em"].(string))
	if err != nil || criada.Location() != time.UTC {
		t.Fatalf("criada_em deveria ser UTC terminando em Z: %v", r["criada_em"])
	}
	if d := time.Since(criada); d < -time.Minute || d > time.Minute {
		t.Fatalf("criada_em %v está longe de agora em UTC (diferença %v)", criada, d)
	}

	// rascunho não vende
	buy := map[string]any{"nome": "Ana", "contato": "65 99999-0000"}
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/numeros/7/comprar", id), "", buy)
	expect(t, "comprar em rascunho", code, 409)

	// outro usuário não pode abrir
	intruso := registerUser(t, app, "intruso@rifa.com")
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/abrir", id), intruso, nil)
	expect(t, "intruso abrindo", code, 403)

	// abrir
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/abrir", id), dono, nil)
	expect(t, "abrir", code, 200)

	// comprar (sem conta)
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/numeros/7/comprar", id), "", buy)
	expect(t, "comprar 7", code, 201)
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/numeros/7/comprar", id), "", buy)
	expect(t, "comprar 7 de novo", code, 409)
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/numeros/201/comprar", id), "", buy)
	expect(t, "comprar 201", code, 400)

	// público não vê contato
	code, det := call(t, app, "GET", fmt.Sprintf("/rifas/%d", id), "", nil)
	expect(t, "detalhe", code, 200)
	vend := det["vendidos"].([]any)[0].(map[string]any)
	if _, temContato := vend["contato"]; temContato {
		t.Fatal("contato vazou no endpoint público")
	}

	// sortear com rifa aberta -> 409
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/sortear", id), dono, nil)
	expect(t, "sortear aberta", code, 409)

	// fechar, não vende mais, sortear
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/fechar", id), dono, nil)
	expect(t, "fechar", code, 200)
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/numeros/8/comprar", id), "", buy)
	expect(t, "comprar fechada", code, 409)
	code, res := call(t, app, "POST", fmt.Sprintf("/rifas/%d/sortear", id), dono, nil)
	expect(t, "sortear", code, 200)
	if res["vencedor"].(map[string]any)["numero"].(float64) != 7 {
		t.Fatalf("só o 7 foi vendido, mas o vencedor foi %v", res["vencedor"])
	}
	code, _ = call(t, app, "POST", fmt.Sprintf("/rifas/%d/sortear", id), dono, nil)
	expect(t, "sortear de novo", code, 409)
}

// TestCompraConcorrente: 50 pessoas tentam comprar o MESMO número ao mesmo
// tempo. Exatamente uma deve conseguir; as outras 49 recebem 409.
func TestCompraConcorrente(t *testing.T) {
	app := setup(t)
	dono := registerUser(t, app, "dono@rifa.com")
	_, r := call(t, app, "POST", "/rifas", dono, map[string]any{"titulo": "t", "premio": "p"})
	id := int(r["id"].(float64))
	call(t, app, "POST", fmt.Sprintf("/rifas/%d/abrir", id), dono, nil)

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := map[int]int{}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, _ := call(t, app, "POST", fmt.Sprintf("/rifas/%d/numeros/42/comprar", id), "",
				map[string]any{"nome": fmt.Sprintf("pessoa %d", i), "contato": "x"})
			mu.Lock()
			results[code]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if results[201] != 1 || results[409] != 49 {
		t.Fatalf("esperava 1x201 e 49x409, veio %v", results)
	}
}

func expect(t *testing.T, what string, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: status %d, esperava %d", what, got, want)
	}
}
