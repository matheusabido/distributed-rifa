package httpapi

import "log"

// Notifier é quem avisa o mundo que algo mudou ("número 42 vendido").
//
// Na API de verdade quem implementa é o realtime.Cluster, que:
//   - manda o evento por websocket para os navegadores desta API;
//   - chama Publish (gRPC) nas outras APIs.
//
// Nos testes usamos o LogNotifier, que só escreve no log.
//
// As rotas não sabem nada disso: só chamam Notify. Por isso dá para trocar a
// implementação sem mexer nelas.
type Notifier interface {
	Notify(eventType string, raffleID int64, payload any)
}

type LogNotifier struct{}

func (LogNotifier) Notify(eventType string, raffleID int64, payload any) {
	log.Printf("[evento] %s rifa=%d", eventType, raffleID)
}

// Tipos de evento (os mesmos do comentário em proto/rifa.proto).
const (
	EventRaffleUpdated = "raffle_updated"
	EventRaffleStatus  = "raffle_status"
	EventTicketBought  = "ticket_bought"
	EventRaffleDrawn   = "raffle_drawn"
)
