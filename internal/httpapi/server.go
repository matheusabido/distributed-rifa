// Package httpapi monta as rotas HTTP com Fiber.
package httpapi

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"rifa/internal/auth"
	"rifa/internal/realtime"
	"rifa/internal/store"
)

type Server struct {
	store  *store.Store
	auth   *auth.Auth
	notify Notifier
	nodeID int
	hub    *realtime.Hub // navegadores conectados por websocket (pode ser nil nos testes)

	corsOrigins string
	clusterView func() any // estado do cluster (rota GET /cluster)
}

func New(st *store.Store, a *auth.Auth, n Notifier, nodeID int, hub *realtime.Hub) *Server {
	return &Server{store: st, auth: a, notify: n, nodeID: nodeID, hub: hub, corsOrigins: "*"}
}

// WithClusterView liga a rota GET /cluster, que mostra o que esta API sabe
// sobre as outras APIs e os bancos.
func (s *Server) WithClusterView(fn func() any) *Server {
	s.clusterView = fn
	return s
}

// WithCORS define quais origens (sites) podem chamar a API pelo navegador.
// Ex: "http://localhost:5500,https://rifa.exemplo.com". Padrão: "*".
func (s *Server) WithCORS(origins string) *Server {
	if origins != "" {
		s.corsOrigins = origins
	}
	return s
}

// App cria o app Fiber com todas as rotas. Está separado do main para que os
// testes consigam criar o app sem abrir porta de rede.
func (s *Server) App() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: errorHandler})
	app.Use(recover.New()) // um panic vira 500 em vez de derrubar a API
	app.Use(logger.New())

	// CORS: deixa um frontend servido em OUTRO endereço (ex: localhost:5500,
	// Live Server do VS Code) chamar esta API pelo navegador. Sem isso o
	// navegador bloqueia as requisições. CORS_ORIGINS controla quem pode
	// ("*" = qualquer um, bom para desenvolvimento).
	app.Use(cors.New(cors.Config{
		AllowOrigins: s.corsOrigins,
		AllowHeaders: "Content-Type, Authorization",
		AllowMethods: "GET, POST, PUT, OPTIONS",
	}))

	// Cabeçalho que mostra qual réplica respondeu. Útil quando houver várias.
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-API-Node", itoa(s.nodeID))
		return c.Next()
	})

	// --- públicas: qualquer um, sem conta ---
	app.Post("/auth/registrar", s.register)
	app.Post("/auth/login", s.login)
	app.Get("/rifas", s.listRaffles)
	app.Get("/rifas/:id", s.getRaffle)
	app.Post("/rifas/:id/numeros/:numero/comprar", s.buyNumber)

	if s.clusterView != nil {
		app.Get("/cluster", func(c *fiber.Ctx) error { return c.JSON(s.clusterView()) })
	}

	// --- tempo real: o navegador abre ws://.../ws e fica ouvindo eventos ---
	if s.hub != nil {
		app.Use("/ws", func(c *fiber.Ctx) error {
			// Só aceita quem pediu "upgrade" para websocket.
			if websocket.IsWebSocketUpgrade(c) {
				return c.Next()
			}
			return fiber.ErrUpgradeRequired
		})
		app.Get("/ws", websocket.New(s.hub.Handle))
	}

	// --- protegidas: precisam de "Authorization: Bearer <token>" ---
	priv := app.Group("", s.auth.Middleware())
	priv.Get("/minhas-rifas", s.myRaffles)
	priv.Post("/rifas", s.createRaffle)
	priv.Put("/rifas/:id", s.updateRaffle)
	priv.Post("/rifas/:id/abrir", s.openRaffle)
	priv.Post("/rifas/:id/fechar", s.closeRaffle)
	priv.Post("/rifas/:id/sortear", s.drawRaffle)
	priv.Get("/rifas/:id/compradores", s.listBuyers)

	return app
}
