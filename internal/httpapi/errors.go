package httpapi

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"

	"rifa/internal/auth"
	"rifa/internal/store"
)

// errorHandler é chamado pelo Fiber sempre que um handler devolve erro.
// Ele traduz os erros de regra de negócio em status HTTP e responde sempre
// no mesmo formato: {"erro": "mensagem"}.
func errorHandler(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	msg := "erro interno"

	var fe *fiber.Error
	switch {
	case errors.As(err, &fe):
		status, msg = fe.Code, fe.Message
	case errors.Is(err, store.ErrNotFound):
		status, msg = fiber.StatusNotFound, err.Error()
	case errors.Is(err, store.ErrForbidden):
		status, msg = fiber.StatusForbidden, err.Error()
	case errors.Is(err, auth.ErrInvalidLogin):
		status, msg = fiber.StatusUnauthorized, err.Error()
	case errors.Is(err, store.ErrNumberTaken),
		errors.Is(err, store.ErrEmailTaken),
		errors.Is(err, store.ErrInvalidState),
		errors.Is(err, store.ErrNoTickets):
		status, msg = fiber.StatusConflict, err.Error()
	case errors.Is(err, store.ErrNumberRange):
		status, msg = fiber.StatusBadRequest, err.Error()
	case isDBUnavailable(err):
		// Banco fora do ar (ou trocando de primário): pede para tentar de novo.
		log.Printf("banco indisponível em %s %s: %v", c.Method(), c.Path(), err)
		status, msg = fiber.StatusServiceUnavailable, "banco de dados indisponível, tente de novo em instantes"
	default:
		// Erro inesperado (banco fora do ar, bug...): loga o detalhe, mas não
		// mostra para o cliente.
		log.Printf("erro em %s %s: %v", c.Method(), c.Path(), err)
	}
	return c.Status(status).JSON(fiber.Map{"erro": msg})
}

// isDBUnavailable reconhece erros de "não consegui falar com o banco":
// falha ao conectar, ou o banco em modo somente leitura (réplica).
func isDBUnavailable(err error) bool {
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return true
	}
	var pgErr *pgconn.PgError
	// 25006 = read_only_sql_transaction (escreveu numa réplica)
	// 57P01 = admin_shutdown (o banco foi desligado no meio)
	if errors.As(err, &pgErr) && (pgErr.Code == "25006" || pgErr.Code == "57P01") {
		return true
	}
	return pgconn.SafeToRetry(err)
}

func badRequest(msg string) error {
	return fiber.NewError(fiber.StatusBadRequest, msg)
}
