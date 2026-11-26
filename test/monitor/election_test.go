package monitor_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"rifa/gen/rifapb"
	"rifa/internal/config"
	"rifa/internal/monitor"
)

// node é uma API de teste: monitor + servidor gRPC numa porta local.
type node struct {
	mon    *monitor.Monitor
	srv    *grpc.Server
	cancel context.CancelFunc
}

func startNode(t *testing.T, id int, addrs map[int]string, lis net.Listener) *node {
	t.Helper()
	cfg := config.API{NodeID: id}
	for pid, a := range addrs {
		if pid != id {
			cfg.Peers = append(cfg.Peers, config.Peer{ID: pid, Addr: a})
		}
	}
	mon, err := monitor.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	rifapb.RegisterMonitorServer(srv, mon)
	go srv.Serve(lis)
	ctx, cancel := context.WithCancel(context.Background())
	go mon.Run(ctx)
	return &node{mon: mon, srv: srv, cancel: cancel}
}

func (n *node) stop() { n.cancel(); n.srv.Stop() }

func waitMaster(t *testing.T, nodes []*node, want int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range nodes {
			if n.mon.MasterID() != want {
				ok = false
			}
		}
		if ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, n := range nodes {
		t.Logf("api%d acha que a mestre é api%d", n.mon.View().Me, n.mon.MasterID())
	}
	t.Fatalf("esperava todas concordarem que a mestre é api%d", want)
}

func listen(t *testing.T, addr string) net.Listener {
	t.Helper()
	var lis net.Listener
	var err error
	for i := 0; i < 50; i++ { // a porta pode demorar a liberar
		if lis, err = net.Listen("tcp", addr); err == nil {
			return lis
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal(err)
	return nil
}

// Sobe 3 APIs, derruba a mestre, confere a eleição e a volta da api1.
func TestEleicaoBully(t *testing.T) {
	if testing.Short() {
		t.Skip("teste lento (espera os horários de checagem)")
	}
	lis := map[int]net.Listener{}
	addrs := map[int]string{}
	for id := 1; id <= 3; id++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		lis[id], addrs[id] = l, l.Addr().String()
	}
	n1 := startNode(t, 1, addrs, lis[1])
	n2 := startNode(t, 2, addrs, lis[2])
	n3 := startNode(t, 3, addrs, lis[3])
	defer n2.stop()
	defer n3.stop()

	waitMaster(t, []*node{n1, n2, n3}, 1, 5*time.Second)

	n1.stop()
	waitMaster(t, []*node{n2, n3}, 2, 15*time.Second)

	n1 = startNode(t, 1, addrs, listen(t, addrs[1]))
	defer n1.stop()
	waitMaster(t, []*node{n1, n2, n3}, 1, 5*time.Second)
	fmt.Println("ok: 1 -> 2 -> 1")
}
