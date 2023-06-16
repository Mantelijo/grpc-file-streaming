package protobuf

//go:generate protoc --go_out=../internal/gen --go_opt=paths=source_relative --go-grpc_out=../internal/gen --go-grpc_opt=paths=source_relative service.proto
