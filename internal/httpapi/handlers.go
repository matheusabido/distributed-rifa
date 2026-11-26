package httpapi

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"rifa/internal/auth"
	"rifa/internal/store"
)

// Cada handler segue o mesmo roteiro:
//   1. ler e validar a entrada (corpo JSON, parâmetros da URL);
//   2. chamar o store (que fala com o banco);
//   3. se gravou algo, avisar com s.notify.Notify(...);
//   4. responder JSON.
// Erros são só "return err": o errorHandler (errors.go) cuida do status HTTP.

// ---------------------------------------------------------------------------
// Autenticação
// ---------------------------------------------------------------------------

type credentials struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// POST /auth/registrar  {"email": "...", "senha": "..."}
func (s *Server) register(c *fiber.Ctx) error {
	var in credentials
	if err := c.BodyParser(&in); err != nil {
		return badRequest("JSON inválido")
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(in.Email, "@") {
		return badRequest("e-mail inválido")
	}
	if len(in.Senha) < 6 {
		return badRequest("a senha precisa ter pelo menos 6 caracteres")
	}

	hash, err := auth.HashPassword(in.Senha)
	if err != nil {
		return err
	}
	u, err := s.store.CreateUser(c.Context(), in.Email, hash)
	if err != nil {
		return err
	}
	return s.respondWithToken(c, fiber.StatusCreated, u.ID)
}

// POST /auth/login  {"email": "...", "senha": "..."}  ->  {"token": "..."}
func (s *Server) login(c *fiber.Ctx) error {
	var in credentials
	if err := c.BodyParser(&in); err != nil {
		return badRequest("JSON inválido")
	}
	u, err := s.store.GetUserByEmail(c.Context(), strings.ToLower(strings.TrimSpace(in.Email)))
	// Mesma mensagem para "e-mail não existe" e "senha errada": assim ninguém
	// descobre quais e-mails estão cadastrados.
	if err == store.ErrNotFound || (err == nil && !auth.CheckPassword(u.PasswordHash, in.Senha)) {
		return auth.ErrInvalidLogin
	}
	if err != nil {
		return err
	}
	return s.respondWithToken(c, fiber.StatusOK, u.ID)
}

func (s *Server) respondWithToken(c *fiber.Ctx, status int, userID int64) error {
	token, err := s.auth.NewToken(userID)
	if err != nil {
		return err
	}
	return c.Status(status).JSON(fiber.Map{"token": token, "usuario_id": userID})
}

// ---------------------------------------------------------------------------
// Rotas públicas
// ---------------------------------------------------------------------------

// GET /rifas            -> rifas abertas
// GET /rifas?status=drawn -> rifas sorteadas, etc.
func (s *Server) listRaffles(c *fiber.Ctx) error {
	status := c.Query("status", store.StatusOpen)
	if status == store.StatusDraft {
		// rascunho é só do dono; aparece em /minhas-rifas
		return badRequest("status inválido")
	}
	raffles, err := s.store.ListRaffles(c.Context(), status, 0)
	if err != nil {
		return err
	}
	return c.JSON(raffles)
}

// GET /rifas/:id -> a rifa + os números vendidos (só nome, sem contato)
func (s *Server) getRaffle(c *fiber.Ctx) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	r, err := s.store.GetRaffle(c.Context(), id)
	if err != nil {
		return err
	}
	tickets, err := s.store.ListTickets(c.Context(), id)
	if err != nil {
		return err
	}
	// O contato (telefone/e-mail) do comprador é dado pessoal: no público
	// mostramos só o nome. O dono vê tudo em /rifas/:id/compradores.
	for i := range tickets {
		tickets[i].BuyerContact = ""
	}
	return c.JSON(fiber.Map{"rifa": r, "vendidos": tickets})
}

type buyInput struct {
	Nome    string `json:"nome"`
	Contato string `json:"contato"`
}

// POST /rifas/:id/numeros/:numero/comprar  {"nome": "...", "contato": "..."}
func (s *Server) buyNumber(c *fiber.Ctx) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	number, err := c.ParamsInt("numero")
	if err != nil {
		return badRequest("número inválido")
	}
	var in buyInput
	if err := c.BodyParser(&in); err != nil {
		return badRequest("JSON inválido")
	}
	in.Nome, in.Contato = strings.TrimSpace(in.Nome), strings.TrimSpace(in.Contato)
	if in.Nome == "" || in.Contato == "" {
		return badRequest("informe nome e contato")
	}

	t, err := s.store.BuyNumber(c.Context(), id, number, in.Nome, in.Contato)
	if err != nil {
		return err
	}
	// O evento vai para todo mundo, então leva só o nome, não o contato.
	s.notify.Notify(EventTicketBought, id, fiber.Map{
		"rifa_id": id, "numero": t.Number, "nome": t.BuyerName,
	})
	return c.Status(fiber.StatusCreated).JSON(t)
}

// ---------------------------------------------------------------------------
// Rotas do dono (protegidas)
// ---------------------------------------------------------------------------

// GET /minhas-rifas -> todas as rifas do usuário logado, inclusive rascunhos
func (s *Server) myRaffles(c *fiber.Ctx) error {
	raffles, err := s.store.ListRaffles(c.Context(), "", auth.UserID(c))
	if err != nil {
		return err
	}
	return c.JSON(raffles)
}

type raffleBody struct {
	Titulo        string `json:"titulo"`
	Descricao     string `json:"descricao"`
	Premio        string `json:"premio"`
	PrecoCentavos int64  `json:"preco_centavos"`
	TotalNumeros  int    `json:"total_numeros"`
}

func parseRaffleBody(c *fiber.Ctx) (store.RaffleInput, error) {
	var b raffleBody
	if err := c.BodyParser(&b); err != nil {
		return store.RaffleInput{}, badRequest("JSON inválido")
	}
	in := store.RaffleInput{
		Title:        strings.TrimSpace(b.Titulo),
		Description:  strings.TrimSpace(b.Descricao),
		Prize:        strings.TrimSpace(b.Premio),
		PriceCents:   b.PrecoCentavos,
		TotalNumbers: b.TotalNumeros,
	}
	if in.TotalNumbers == 0 {
		in.TotalNumbers = 200 // padrão da nossa rifa
	}
	switch {
	case in.Title == "":
		return in, badRequest("informe o título")
	case in.Prize == "":
		return in, badRequest("informe o prêmio")
	case in.PriceCents < 0:
		return in, badRequest("preço não pode ser negativo")
	case in.TotalNumbers < 1 || in.TotalNumbers > 10000:
		return in, badRequest("total de números deve ficar entre 1 e 10000")
	}
	return in, nil
}

// POST /rifas  {"titulo": "...", "premio": "...", "preco_centavos": 500}
// A rifa nasce como rascunho (draft). Para vender, chame /abrir.
func (s *Server) createRaffle(c *fiber.Ctx) error {
	in, err := parseRaffleBody(c)
	if err != nil {
		return err
	}
	r, err := s.store.CreateRaffle(c.Context(), auth.UserID(c), in)
	if err != nil {
		return err
	}
	// Sem evento aqui: rascunho é privado do dono. O público fica sabendo da
	// rifa quando ela for aberta (evento raffle_status).
	return c.Status(fiber.StatusCreated).JSON(r)
}

// PUT /rifas/:id  (mesmo corpo do POST)
func (s *Server) updateRaffle(c *fiber.Ctx) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	in, err := parseRaffleBody(c)
	if err != nil {
		return err
	}
	r, err := s.store.UpdateRaffle(c.Context(), id, auth.UserID(c), in)
	if err != nil {
		return err
	}
	if r.Status != store.StatusDraft { // rascunho continua privado
		s.notify.Notify(EventRaffleUpdated, r.ID, r)
	}
	return c.JSON(r)
}

func (s *Server) openRaffle(c *fiber.Ctx) error  { return s.setStatus(c, store.StatusOpen) }
func (s *Server) closeRaffle(c *fiber.Ctx) error { return s.setStatus(c, store.StatusClosed) }

func (s *Server) setStatus(c *fiber.Ctx, status string) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	r, err := s.store.SetStatus(c.Context(), id, auth.UserID(c), status)
	if err != nil {
		return err
	}
	s.notify.Notify(EventRaffleStatus, r.ID, r)
	return c.JSON(r)
}

// POST /rifas/:id/sortear  (a rifa precisa estar fechada)
func (s *Server) drawRaffle(c *fiber.Ctx) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	r, winner, err := s.store.DrawWinner(c.Context(), id, auth.UserID(c))
	if err != nil {
		return err
	}
	// Para o público vai só o número e o nome do vencedor.
	s.notify.Notify(EventRaffleDrawn, r.ID, fiber.Map{
		"rifa_id": r.ID, "numero": winner.Number, "nome": winner.BuyerName,
	})
	// O dono recebe também o contato, para entregar o prêmio.
	return c.JSON(fiber.Map{"rifa": r, "vencedor": winner})
}

// GET /rifas/:id/compradores -> lista completa (com contato), só para o dono
func (s *Server) listBuyers(c *fiber.Ctx) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	r, err := s.store.GetRaffle(c.Context(), id)
	if err != nil {
		return err
	}
	if r.OwnerID != auth.UserID(c) {
		return store.ErrForbidden
	}
	tickets, err := s.store.ListTickets(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(tickets)
}

// ---------------------------------------------------------------------------

func paramID(c *fiber.Ctx, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Params(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("id inválido")
	}
	return id, nil
}

func itoa(n int) string { return strconv.Itoa(n) }
