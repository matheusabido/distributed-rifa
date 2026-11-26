// apinode: uma réplica da API HTTP (Fiber).
//
// Cada réplica abre DUAS portas:
//
//	HTTP_ADDR (:8080) -> navegadores: rotas REST + websocket (/ws) + /cluster
//	GRPC_ADDR (:9000) -> outras APIs: Broadcast (eventos) e Monitor (saúde)
//
// E roda o MONITOR em segundo plano (ver internal/monitor): a API MESTRE
// checa as outras APIs e os bancos e, se o primário cai, promove a réplica
// depois de ganhar a votação; as SEGUIDORAS só checam se a mestre responde
// e, se não, fazem uma eleição (vence a de menor ID).
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"rifa/gen/rifapb"
	"rifa/internal/auth"
	"rifa/internal/config"
	"rifa/internal/httpapi"
	"rifa/internal/monitor"
	"rifa/internal/realtime"
	"rifa/internal/store"
)

func main() {
	cfg := config.Load()
	log.SetPrefix(fmt.Sprintf("[api %d] ", cfg.NodeID))
	log.SetFlags(log.Ltime | log.LUTC) // horário dos logs em UTC

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Banco: pool que sempre procura o primário.
	st, err := store.Connect(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("configuração do banco inválida: %v", err)
	}
	defer st.Close()

	// 2. Tempo real: hub de websocket + clientes gRPC (Broadcast) das outras APIs.
	hub := realtime.NewHub()
	var peerAddrs []string
	for _, p := range cfg.Peers {
		peerAddrs = append(peerAddrs, p.Addr)
	}
	cluster, err := realtime.NewCluster(cfg.NodeID, hub, peerAddrs)
	if err != nil {
		log.Fatalf("gRPC: %v", err)
	}

	// 3. Monitor: saúde das APIs e dos bancos.
	mon, err := monitor.New(cfg)
	if err != nil {
		log.Fatalf("monitor: %v", err)
	}
	mon.OnPrimaryChange = func() {
		log.Println("[banco] o primário mudou: reabrindo as conexões")
		st.Reset()
	}
	go mon.Run(ctx)

	// 4. Servidor gRPC: recebe chamadas das outras APIs.
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("gRPC: %v", err)
	}
	grpcServer := grpc.NewServer()
	rifapb.RegisterBroadcastServer(grpcServer, cluster)
	rifapb.RegisterMonitorServer(grpcServer, mon)
	go func() {
		log.Printf("gRPC em %s, outras APIs: %v", cfg.GRPCAddr, cfg.Peers)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC: %v", err)
		}
	}()

	// 5. Servidor HTTP.
	srv := httpapi.New(st, auth.New(cfg.JWTSecret), cluster, cfg.NodeID, hub).
		WithCORS(cfg.CORSOrigins).
		WithClusterView(func() any { return mon.View() })
	app := srv.App()

	// 6. Desliga com calma no Ctrl+C / docker stop.
	go func() {
		<-ctx.Done()
		log.Println("desligando...")
		grpcServer.GracefulStop()
		_ = app.ShutdownWithTimeout(5 * time.Second)
	}()

	log.Printf("HTTP em %s", cfg.HTTPAddr)
	if err := app.Listen(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
