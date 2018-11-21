//go:generate protoc -I../../golang/protobuf/ptypes/empty --dart_out=grpc:app/lib/src/google/protobuf empty.proto
//go:generate protoc -I. --go_out=plugins=grpc:../../.. --dart_out=grpc:app/lib/src protos/authstore.proto
//go:generate protoc -I. --go_out=plugins=grpc:../../.. --dart_out=grpc:app/lib/src protos/config.proto
//go:generate protoc -I. --go_out=plugins=grpc:../../.. --dart_out=grpc:app/lib/src protos/grpc.proto
//go:generate protoc-go-inject-tag -input=config/config.pb.go
//go:generate protoc-go-inject-field -input=config/config.pb.go
package hybrid