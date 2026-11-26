package realtime

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"rifa/gen/rifapb"
	"rifa/internal/peerconn"
)

// Message é o JSON que chega no navegador pelo websocket.
type Message struct {
	Type     string `json:"tipo"`    // ex: "ticket_bought"
	RaffleID int64  `json:"rifa_id"` // qual rifa mudou
	Data     any    `json:"dados"`   // detalhes (número, nome...)
	Origin   int    `json:"origem"`  // qual API gerou (para você ver no navegador)
}

// Cluster junta as duas pontas do tempo real:
//
//	Notify  -> manda para os navegadores DESTA API (Hub)
//	        -> e chama Publish (gRPC) nas OUTRAS APIs
//
//	Publish -> (chamado por outra API via gRPC) manda para os navegadores
//	           desta API
//
// Ele implementa ao mesmo tempo:
//   - httpapi.Notifier (o método Notify, usado pelas rotas);
//   - rifapb.BroadcastServer (o método Publish, usado pelo gRPC).
type Cluster struct {
	rifapb.UnimplementedBroadcastServer

	nodeID int
	hub    *Hub
	peers  []peer
}

type peer struct {
	addr   string
	client rifapb.BroadcastClient
}

// NewCluster prepara os clientes gRPC das outras APIs.
//
// peerconn.Dial NÃO conecta na hora: ele conecta na primeira chamada e
// reconecta sozinho se a outra API cair e voltar. Então a ordem em que as
// APIs sobem não importa.
func NewCluster(nodeID int, hub *Hub, peerAddrs []string) (*Cluster, error) {
	c := &Cluster{nodeID: nodeID, hub: hub}
	for _, addr := range peerAddrs {
		conn, err := peerconn.Dial(addr)
		if err != nil {
			return nil, err
		}
		c.peers = append(c.peers, peer{addr: addr, client: rifapb.NewBroadcastClient(conn)})
	}
	return c, nil
}

// Notify é chamado pelas rotas HTTP depois de uma escrita que deu certo.
func (c *Cluster) Notify(eventType string, raffleID int64, data any) {
	msg, err := json.Marshal(Message{Type: eventType, RaffleID: raffleID, Data: data, Origin: c.nodeID})
	if err != nil {
		log.Printf("[evento] erro ao montar JSON: %v", err)
		return
	}

	// 1. navegadores desta API
	c.hub.Broadcast(msg)

	// 2. outras APIs, em paralelo e em segundo plano: a resposta HTTP de
	//    quem comprou não espera o aviso chegar nas outras APIs.
	ev := &rifapb.Event{
		Type:        eventType,
		RaffleId:    raffleID,
		PayloadJson: string(msg),
		OriginApiId: int32(c.nodeID),
	}
	go c.publishToPeers(ev)
}

func (c *Cluster) publishToPeers(ev *rifapb.Event) {
	var wg sync.WaitGroup
	for _, p := range c.peers {
		wg.Add(1)
		go func(p peer) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := p.client.Publish(ctx, ev); err != nil {
				// Melhor esforço: se a outra API está fora do ar, ela não tem
				// navegadores conectados mesmo. Quando voltar, os navegadores
				// reconectam e buscam o estado atual pela API HTTP.
				log.Printf("[grpc] falha ao avisar %s: %v", p.addr, err)
			}
		}(p)
	}
	wg.Wait()
}

// Publish é o lado SERVIDOR do gRPC: outra API está nos avisando de algo.
// Só repassamos para os navegadores conectados aqui. NÃO reenviamos para as
// outras APIs, senão o evento ficaria rodando em círculo.
func (c *Cluster) Publish(ctx context.Context, ev *rifapb.Event) (*rifapb.PublishResponse, error) {
	log.Printf("[grpc] evento %s da api %d, repassando para %d navegador(es)",
		ev.Type, ev.OriginApiId, c.hub.Count())
	c.hub.Broadcast([]byte(ev.PayloadJson))
	return &rifapb.PublishResponse{}, nil
}
