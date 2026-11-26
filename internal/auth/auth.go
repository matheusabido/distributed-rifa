// Package auth cuida de senha (bcrypt) e de token (JWT).
//
// Fluxo:
//
//  1. Cadastro: a senha vira um hash bcrypt e só o hash vai para o banco.
//  2. Login: comparamos a senha digitada com o hash. Se bater, geramos um
//     token JWT assinado com JWT_SECRET, válido por 24h.
//  3. Rotas protegidas: o cliente manda "Authorization: Bearer <token>". O
//     middleware confere a assinatura e descobre o ID do usuário.
//
// Por que JWT? O token carrega o ID do usuário e uma assinatura. Qualquer
// réplica da API que conheça JWT_SECRET consegue validar sozinha, sem
// consultar o banco nem as outras APIs.
package auth

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const tokenTTL = 24 * time.Hour

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type Auth struct {
	secret []byte
}

func New(secret string) *Auth { return &Auth{secret: []byte(secret)} }

// NewToken gera um JWT com o ID do usuário no campo "sub" (subject).
func (a *Auth) NewToken(userID int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// ParseToken confere assinatura e validade e devolve o ID do usuário.
func (a *Auth) ParseToken(token string) (int64, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"})) // recusa tokens com outro algoritmo
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(claims.Subject, 10, 64)
}

// Middleware protege uma rota: sem token válido, responde 401 e nem chega no
// handler. Com token válido, guarda o ID em c.Locals("userID").
func (a *Auth) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "faltou o header Authorization: Bearer <token>")
		}
		userID, err := a.ParseToken(token)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "token inválido ou expirado")
		}
		c.Locals("userID", userID)
		return c.Next()
	}
}

// UserID lê o ID que o Middleware guardou.
func UserID(c *fiber.Ctx) int64 {
	id, _ := c.Locals("userID").(int64)
	return id
}

var ErrInvalidLogin = errors.New("e-mail ou senha inválidos")
