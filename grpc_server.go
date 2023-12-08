/**
 * @Author: lidonglin
 * @Description:
 * @File:  grpc_server
 * @Version: 1.0.0
 * @Date: 2023/12/07 20:56
 */

package tgserver

import (
	"context"
	"fmt"
	"net"

	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
)

type GrpcServer struct {
	grpcOption GrpcOption

	grpcServer *grpc.Server
}

func NewGrpcServer(ctx context.Context, grpcOption GrpcOption, grpcPort int) (*GrpcServer, error) {
	grpcServer := &GrpcServer{
		grpcOption: grpcOption,
	}

	if len(grpcServer.grpcOption.registrars) == 0 {
		return nil, fmt.Errorf("no grpc service registrar")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		return nil, err
	}

	options := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			latencyServerInterceptor,
			prometheusServerInterceptor,
			logServerInterceptor,
			grpcrecovery.UnaryServerInterceptor(
				grpcrecovery.WithRecoveryHandlerContext(recoveryHandler),
			),
			errorServerInterceptor,
		),
	}

	options = append(options, grpcServer.grpcOption.options...)

	ReplaceGrpcLoggerV2()

	grpcServer.grpcServer = grpc.NewServer(options...)

	for _, registrar := range grpcServer.grpcOption.registrars {
		registrar(grpcServer.grpcServer)
	}

	go func() {
		err := grpcServer.grpcServer.Serve(listener)
		if err != nil {
			return
		}
	}()

	return grpcServer, nil
}

func (p *GrpcServer) Shutdown(ctx context.Context) {
	if p != nil {
		return
	}

	if p.grpcServer != nil {
		p.grpcServer.GracefulStop()
	}
}
