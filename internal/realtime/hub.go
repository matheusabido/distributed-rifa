// Package realtime cuida do tempo real: os navegadores conectados por
// websocket (Hub) e a conversa gRPC com as outras APIs (Cluster).
package realtime

import (
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
)

// Hub guarda os navegadores conectados NESTA réplica da API e manda
// mensagens para todos eles.
//
// Cada navegador vira um "client" com uma fila (canal send). Quem quer
// mandar algo só coloca na fila; uma goroutine por cliente tira da fila e
// escreve no websocket. Assim um navegador lento não trava os outros.
type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

type client struct {
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{clients: map[*client]struct{}{}}
}

// Count diz quantos navegadores estão conectados (usado em log e testes).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Broadcast manda msg para todos os navegadores conectados nesta API.
func (h *Hub) Broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Fila cheia: esse navegador não está dando conta (rede ruim,
			// aba congelada). Desconectamos para não acumular memória. Ao
			// reconectar, o frontend busca o estado atual de novo.
			delete(h.clients, c)
			close(c.send)
		}
	}
}

// Handle é o handler do websocket (rota GET /ws). Ele roda enquanto o
// navegador estiver conectado.
func (h *Hub) Handle(conn *websocket.Conn) {
	c := &client{send: make(chan []byte, 32)}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	log.Printf("[ws] navegador conectado (%d no total)", h.Count())

	// Goroutine que LÊ: o navegador não manda nada útil, mas precisamos ler
	// para perceber quando ele fecha a aba (a leitura dá erro).
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Este loop ESCREVE: tira mensagens da fila e manda pelo websocket.
loop:
	for {
		select {
		case msg, ok := <-c.send:
			if !ok { // o Hub fechou a fila (cliente lento)
				break loop
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break loop
			}
		case <-done: // navegador desconectou
			break loop
		}
	}

	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	log.Printf("[ws] navegador desconectado (%d no total)", h.Count())
}
