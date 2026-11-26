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
//
// "passthrough:///" faz o nome (ex "api1") ser resolvido a CADA tentativa de
// conexão. Sem isso, o gRPC usa o resolvedor DNS dele, que guarda a resposta
// e só pergunta de novo a cada ~30s: quando um container para, o nome some
// do DNS do Docker, e quando ele volta as outras APIs ficavam até 30s sem
// conseguir falar com ele (e achavam que ele continuava fora do ar).
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///"+addr,
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
