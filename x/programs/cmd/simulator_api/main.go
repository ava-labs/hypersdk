//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./api/service.proto
package main

import (
	context "context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ava-labs/hypersdk/crypto/ed25519"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/cmd"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator_api/api"
	"google.golang.org/grpc"
)

var (
	port     = flag.Int("port", 50051, "The server port")
	logLevel = flag.String("log-level", "info", "log level")
)

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		panic(err)
	}

	opt := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opt...)
	simulator := cmd.NewSimulator(*logLevel)
	if err := simulator.Init(); err != nil {
		panic(err)
	}
	defer simulator.GracefulStop(context.Background())

	api.RegisterSimulatorServer(grpcServer, newSimulatorServer(simulator))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		grpcServer.Serve(lis)
	}()
	<-sigCh
	grpcServer.GracefulStop()

}

type Simulator interface {
	Init() error
	CreateKey(ctx context.Context, name string) (ed25519.PublicKey, error)
}

func newSimulatorServer(simulator Simulator) api.SimulatorServer {
	return &simulatorServer{
		simulator: simulator,
	}
}

type simulatorServer struct {
	simulator Simulator
	api.UnimplementedSimulatorServer
}

func (x *simulatorServer) CreateKey(ctx context.Context, in *api.CreateKeyRequest) (*api.CreateKeyResponse, error) {
	_, err := x.simulator.CreateKey(ctx, in.GetName())
	if err != nil && !errors.Is(err, cmd.ErrDuplicateKeyName) {
		return nil, err
	}
	return &api.CreateKeyResponse{
		Name: in.GetName(),
	}, nil
}
