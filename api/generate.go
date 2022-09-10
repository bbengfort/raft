package api

//go:generate protoc -I=$GOPATH/src/github.com/bbengfort/raft/proto --go_out=. --go_opt=module=github.com/bbengfort/raft/api --go-grpc_out=. --go-grpc_opt=module=github.com/bbengfort/raft/api raft/v1beta1/append.proto raft/v1beta1/client.proto raft/v1beta1/log.proto raft/v1beta1/service.proto raft/v1beta1/time.proto raft/v1beta1/vote.proto
