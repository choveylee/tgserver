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
	"os"
	"os/signal"
	"syscall"

	"github.com/choveylee/tlog"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
)

type GrpcServer struct {
	grpcOption GrpcOption

	grpcServer *grpc.Server
}

func StartGrpcServer(ctx context.Context, grpcOption GrpcOption, grpcPort int) {
	grpcServer := &GrpcServer{
		grpcOption: grpcOption,
	}

	if len(grpcServer.grpcOption.registrars) == 0 {
		tlog.F(ctx).Msg("no grpc service registrar")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		tlog.F(ctx).Err(err).Msgf("start grpc server (%d) err (%v).",
			grpcPort, err)
	}

	options := []grpc.ServerOption{
		// grpc.StatsHandler(otelgrpc.NewServerHandler()),
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

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		err := grpcServer.grpcServer.Serve(listener)
		if err != nil {
			return
		}
	}()

	tlog.I(ctx).Msgf("grpc server started, listen on %d.", grpcPort)

	select {
	case <-ctx.Done():
		err := grpcServer.shutdown(ctx)
		if err != nil {
			tlog.E(ctx).Err(err).Msgf("shutdown grpc server err (%v).",
				err)

			return
		}

		return
	case <-shutdownChan:
		err := grpcServer.shutdown(ctx)
		if err != nil {
			tlog.E(ctx).Err(err).Msgf("shutdown grpc server err (%v).",
				err)

			return
		}
		return
	}
}

func (p *GrpcServer) shutdown(ctx context.Context) error {
	if p != nil {
		return nil
	}

	if p.grpcServer != nil {
		p.grpcServer.GracefulStop()
	}

	return nil
}
