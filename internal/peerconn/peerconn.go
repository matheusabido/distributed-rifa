// Package peerconn cria a conexão gRPC com outra API.
package peerconn

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial prepara a conexão com outra API. Não conecta na hora: conecta na
// primeira chamada e reconecta sozinho.
//
// Por padrão, o gRPC espera cada vez MAIS entre uma tentativa de reconexão e
// outra (até 2 minutos!). Para nós isso é ruim: uma API que voltou ficaria
// parecendo "fora do ar" por muito tempo. Então limitamos a espera a 1s.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  200 * time.Millisecond,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   1 * time.Second,
			},
			MinConnectTimeout: 1 * time.Second,
		}),
	)
}
