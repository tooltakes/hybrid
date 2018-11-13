//go:generate protoc -I. --go_out=plugins=grpc:../../.. protos/authstore.proto
//go:generate protoc -I. --go_out=plugins=grpc:../../.. protos/config.proto
//go:generate protoc -I. --go_out=plugins=grpc:../../.. protos/grpc.proto
package hybrid