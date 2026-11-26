// Package store é a única parte da API que fala SQL.
//
// As rotas HTTP nunca escrevem SQL: elas chamam funções daqui, como
// BuyNumber ou DrawWinner. Assim, se o banco do seu amigo mudar (nome de
// tabela, coluna...), só este arquivo muda.
package store

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Erros de regra de negócio. As rotas HTTP traduzem cada um num status HTTP
// (ver internal/httpapi/errors.go).
var (
	ErrNotFound     = errors.New("não encontrado")
	ErrForbidden    = errors.New("você não é o dono desta rifa")
	ErrInvalidState = errors.New("operação não permitida no status atual da rifa")
	ErrNumberTaken  = errors.New("número já vendido")
	ErrNumberRange  = errors.New("número fora do intervalo da rifa")
	ErrEmailTaken   = errors.New("e-mail já cadastrado")
	ErrNoTickets    = errors.New("nenhum número vendido, não dá para sortear")
)

// Status de uma rifa: draft -> open <-> closed -> drawn
const (
	StatusDraft  = "draft"
	StatusOpen   = "open"
	StatusClosed = "closed"
	StatusDrawn  = "drawn"
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
}

type Raffle struct {
	ID           int64     `json:"id"`
	OwnerID      int64     `json:"dono_id"`
	Title        string    `json:"titulo"`
	Description  string    `json:"descricao"`
	Prize        string    `json:"premio"`
	PriceCents   int64     `json:"preco_centavos"`
	TotalNumbers int       `json:"total_numeros"`
	Status       string    `json:"status"`
	WinnerNumber *int      `json:"numero_sorteado"` // nil (null no JSON) até sortear
	SoldCount    int       `json:"vendidos"`
	CreatedAt    time.Time `json:"criada_em"`
}

type Ticket struct {
	RaffleID     int64     `json:"rifa_id"`
	Number       int       `json:"numero"`
	BuyerName    string    `json:"nome"`
	BuyerContact string    `json:"contato,omitempty"` // só o dono vê
	BoughtAt     time.Time `json:"comprado_em"`
}

// RaffleInput são os campos que o dono pode definir ao criar/editar.
type RaffleInput struct {
	Title        string
	Description  string
	Prize        string
	PriceCents   int64
	TotalNumbers int
}

type Store struct {
	pool *pgxpool.Pool
}

// Connect prepara o pool de conexões. O pool mantém algumas conexões abertas
// e as reaproveita entre requisições (abrir conexão é caro).
//
// Se o banco ainda não estiver de pé, NÃO falha: só avisa no log. O pool
// tenta conectar de novo a cada requisição. Assim a API sobe mesmo que os
// bancos demorem, e continua de pé enquanto um banco reserva é promovido.
func Connect(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	// Toda conexão trabalha em UTC. As colunas são TIMESTAMP sem fuso, então
	// é a sessão que garante que "agora" é sempre gravado em UTC.
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	// Conexões em pé há muito tempo podem estar apontando para um banco que
	// deixou de ser o primário; renovar de tempos em tempos ajuda.
	cfg.MaxConnLifetime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		log.Printf("[banco] ainda sem conexão com o primário: %v", err)
	}
	return &Store{pool: pool}, nil
}

// Reset fecha todas as conexões do pool. As próximas são abertas do zero e,
// graças ao target_session_attrs=read-write, vão para o primário ATUAL.
// O monitor chama isto quando percebe que o primário mudou.
func (s *Store) Reset() { s.pool.Reset() }

func (s *Store) Close() { s.pool.Close() }

// ---------------------------------------------------------------------------
// Usuários
// ---------------------------------------------------------------------------

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	u := User{Email: email, PasswordHash: passwordHash}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, passwordHash,
	).Scan(&u.ID)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// ---------------------------------------------------------------------------
// Rifas: leitura
// ---------------------------------------------------------------------------

// raffleColumns é o SELECT padrão de uma rifa, incluindo quantos números já
// foram vendidos (subconsulta em tickets).
const raffleColumns = `
	r.id, r.owner_id, r.title, r.description, r.prize, r.price_cents,
	r.total_numbers, r.status, r.winner_number, r.created_at,
	(SELECT count(*) FROM tickets t WHERE t.raffle_id = r.id)`

func scanRaffle(row pgx.Row) (Raffle, error) {
	var r Raffle
	err := row.Scan(&r.ID, &r.OwnerID, &r.Title, &r.Description, &r.Prize,
		&r.PriceCents, &r.TotalNumbers, &r.Status, &r.WinnerNumber,
		&r.CreatedAt, &r.SoldCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Raffle{}, ErrNotFound
	}
	return r, err
}

// ListRaffles lista as rifas com um status (ex: "open"). Se ownerID > 0,
// filtra pelas rifas daquele dono (status vazio = todos).
func (s *Store) ListRaffles(ctx context.Context, status string, ownerID int64) ([]Raffle, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+raffleColumns+`
		FROM raffles r
		WHERE ($1 = '' OR r.status = $1)
		  AND ($2 = 0 OR r.owner_id = $2)
		ORDER BY r.created_at DESC`, status, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	raffles := []Raffle{}
	for rows.Next() {
		r, err := scanRaffle(rows)
		if err != nil {
			return nil, err
		}
		raffles = append(raffles, r)
	}
	return raffles, rows.Err()
}

func (s *Store) GetRaffle(ctx context.Context, id int64) (Raffle, error) {
	return scanRaffle(s.pool.QueryRow(ctx,
		`SELECT `+raffleColumns+` FROM raffles r WHERE r.id = $1`, id))
}

// ListTickets devolve os números vendidos de uma rifa, em ordem.
func (s *Store) ListTickets(ctx context.Context, raffleID int64) ([]Ticket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT raffle_id, number, buyer_name, buyer_contact, bought_at
		FROM tickets WHERE raffle_id = $1 ORDER BY number`, raffleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := []Ticket{}
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.RaffleID, &t.Number, &t.BuyerName, &t.BuyerContact, &t.BoughtAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

// ---------------------------------------------------------------------------
// Rifas: escrita do dono
// ---------------------------------------------------------------------------

func (s *Store) CreateRaffle(ctx context.Context, ownerID int64, in RaffleInput) (Raffle, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO raffles (owner_id, title, description, prize, price_cents, total_numbers)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		ownerID, in.Title, in.Description, in.Prize, in.PriceCents, in.TotalNumbers,
	).Scan(&id)
	if err != nil {
		return Raffle{}, err
	}
	return s.GetRaffle(ctx, id)
}

// UpdateRaffle edita título, descrição, prêmio e preço. A quantidade de
// números só pode mudar enquanto a rifa é rascunho (ninguém comprou ainda).
func (s *Store) UpdateRaffle(ctx context.Context, id, ownerID int64, in RaffleInput) (Raffle, error) {
	err := s.ownerTx(ctx, id, ownerID, func(tx pgx.Tx, current Raffle) error {
		if current.Status == StatusDrawn {
			return ErrInvalidState
		}
		total := current.TotalNumbers
		if current.Status == StatusDraft && in.TotalNumbers > 0 {
			total = in.TotalNumbers
		}
		_, err := tx.Exec(ctx, `
			UPDATE raffles
			SET title = $2, description = $3, prize = $4, price_cents = $5, total_numbers = $6
			WHERE id = $1`,
			id, in.Title, in.Description, in.Prize, in.PriceCents, total)
		return err
	})
	if err != nil {
		return Raffle{}, err
	}
	return s.GetRaffle(ctx, id)
}

// SetStatus abre ou fecha a rifa.
//
//	abrir:  draft  -> open,  ou closed -> open (reabrir)
//	fechar: open   -> closed
func (s *Store) SetStatus(ctx context.Context, id, ownerID int64, newStatus string) (Raffle, error) {
	err := s.ownerTx(ctx, id, ownerID, func(tx pgx.Tx, current Raffle) error {
		allowed := map[string][]string{
			StatusOpen:   {StatusDraft, StatusClosed},
			StatusClosed: {StatusOpen},
		}
		if !contains(allowed[newStatus], current.Status) {
			return ErrInvalidState
		}
		_, err := tx.Exec(ctx, `UPDATE raffles SET status = $2 WHERE id = $1`, id, newStatus)
		return err
	})
	if err != nil {
		return Raffle{}, err
	}
	return s.GetRaffle(ctx, id)
}

// DrawWinner sorteia um número entre os VENDIDOS. Só com a rifa fechada.
//
// O número aleatório é escolhido aqui na API (crypto/rand) e o resultado é
// gravado no banco. Assim o mestre e as réplicas guardam o mesmo vencedor.
func (s *Store) DrawWinner(ctx context.Context, id, ownerID int64) (Raffle, Ticket, error) {
	var winner Ticket
	err := s.ownerTx(ctx, id, ownerID, func(tx pgx.Tx, current Raffle) error {
		if current.Status != StatusClosed {
			return ErrInvalidState
		}
		rows, err := tx.Query(ctx, `
			SELECT raffle_id, number, buyer_name, buyer_contact, bought_at
			FROM tickets WHERE raffle_id = $1 ORDER BY number`, id)
		if err != nil {
			return err
		}
		tickets, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Ticket, error) {
			var t Ticket
			err := r.Scan(&t.RaffleID, &t.Number, &t.BuyerName, &t.BuyerContact, &t.BoughtAt)
			return t, err
		})
		if err != nil {
			return err
		}
		if len(tickets) == 0 {
			return ErrNoTickets
		}

		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(tickets))))
		if err != nil {
			return err
		}
		winner = tickets[i.Int64()]

		_, err = tx.Exec(ctx,
			`UPDATE raffles SET status = $2, winner_number = $3 WHERE id = $1`,
			id, StatusDrawn, winner.Number)
		return err
	})
	if err != nil {
		return Raffle{}, Ticket{}, err
	}
	r, err := s.GetRaffle(ctx, id)
	return r, winner, err
}

// ownerTx é o "molde" de toda operação do dono:
//
//  1. abre uma transação;
//  2. lê a rifa com FOR UPDATE (trava a linha até o fim da transação, então
//     ninguém muda a rifa no meio do caminho);
//  3. confere se quem pediu é o dono;
//  4. roda a função fn;
//  5. se fn deu certo, COMMIT; se deu erro, ROLLBACK (nada é gravado).
func (s *Store) ownerTx(ctx context.Context, id, ownerID int64, fn func(pgx.Tx, Raffle) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		current, err := scanRaffle(tx.QueryRow(ctx,
			`SELECT `+raffleColumns+` FROM raffles r WHERE r.id = $1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if current.OwnerID != ownerID {
			return ErrForbidden
		}
		return fn(tx, current)
	})
}

// ---------------------------------------------------------------------------
// Compra (sem conta)
// ---------------------------------------------------------------------------

// BuyNumber compra um número. É aqui que mora o "lock durante a escrita".
//
// Duas proteções trabalham juntas:
//
//   - SELECT ... FOR SHARE na rifa: várias compras podem acontecer ao mesmo
//     tempo (todas pegam o lock "compartilhado"), mas o dono não consegue
//     FECHAR a rifa no meio de uma compra, porque fechar precisa do lock
//     exclusivo (FOR UPDATE) e espera as compras terminarem.
//
//   - PRIMARY KEY (raffle_id, number) em tickets: se duas pessoas compram o
//     mesmo número ao mesmo tempo, o Postgres deixa só um INSERT passar. O
//     outro recebe erro de unicidade, que viramos ErrNumberTaken.
func (s *Store) BuyNumber(ctx context.Context, raffleID int64, number int, name, contact string) (Ticket, error) {
	t := Ticket{RaffleID: raffleID, Number: number, BuyerName: name, BuyerContact: contact}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		var total int
		err := tx.QueryRow(ctx,
			`SELECT status, total_numbers FROM raffles WHERE id = $1 FOR SHARE`, raffleID,
		).Scan(&status, &total)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != StatusOpen {
			return ErrInvalidState
		}
		if number < 1 || number > total {
			return ErrNumberRange
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO tickets (raffle_id, number, buyer_name, buyer_contact)
			VALUES ($1, $2, $3, $4) RETURNING bought_at`,
			raffleID, number, name, contact,
		).Scan(&t.BoughtAt)
		if isUniqueViolation(err) {
			return ErrNumberTaken
		}
		return err
	})
	return t, err
}

// ---------------------------------------------------------------------------
// utilidades
// ---------------------------------------------------------------------------

// isUniqueViolation detecta o erro 23505 do Postgres ("já existe uma linha
// com essa chave").
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
