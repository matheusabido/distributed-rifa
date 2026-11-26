# Gera o código Go a partir do .proto.
# Precisa de: protoc, protoc-gen-go e protoc-gen-go-grpc no PATH.
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
proto:
	protoc --go_out=. --go_opt=module=rifa \
	       --go-grpc_out=. --go-grpc_opt=module=rifa \
	       proto/rifa.proto

build:
	go build -o bin/apinode ./cmd/apinode

.PHONY: proto build
