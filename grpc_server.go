package tgserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/choveylee/tlog"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

const defaultGracefulShutdownTimeout = 30 * time.Second

type grpcLifecycleServer interface {
	Serve(net.Listener) error
	GracefulStop()
	Stop()
}

// GrpcServer contains the gRPC server instance and the options used to construct it.
type GrpcServer struct {
	grpcOption GrpcOption

	grpcServer *grpc.Server
}

func defaultServerOptions() []grpc.ServerOption {
	recoveryOptions := grpcrecovery.WithRecoveryHandlerContext(recoveryHandler)

	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			latencyServerInterceptor,
			prometheusServerInterceptor,
			logServerInterceptor,
			grpcrecovery.UnaryServerInterceptor(recoveryOptions),
			errorServerInterceptor,
		),
		grpc.ChainStreamInterceptor(
			latencyStreamServerInterceptor,
			prometheusStreamServerInterceptor,
			logStreamServerInterceptor,
			grpcrecovery.StreamServerInterceptor(recoveryOptions),
			errorStreamServerInterceptor,
		),
	}
}

// StartGrpcServer starts a gRPC server on grpcPort and serves requests until ctx is canceled,
// Serve returns an unexpected error, or the process receives SIGINT or SIGTERM. Shutdown first
// attempts a graceful drain and then forces Stop if the drain does not complete within the
// shutdown timeout. Signal registration is released with [signal.Stop] before the function
// returns, so repeated invocations do not accumulate handlers.
func StartGrpcServer(ctx context.Context, grpcOption GrpcOption, grpcPort int) {
	grpcServer := &GrpcServer{
		grpcOption: grpcOption,
	}

	if len(grpcServer.grpcOption.registrars) == 0 {
		tlog.F(ctx).Msg("No gRPC service registrars have been configured.")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		tlog.F(ctx).Err(err).Msgf("Failed to start the gRPC server on port %d: %v.",
			grpcPort, err)
		return
	}

	options := defaultServerOptions()
	options = append(options, grpcServer.grpcOption.options...)

	ReplaceGrpcLoggerV2()

	grpcServer.grpcServer = grpc.NewServer(options...)

	for _, registrar := range grpcServer.grpcOption.registrars {
		registrar(grpcServer.grpcServer)
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownChan)

	tlog.I(ctx).Msgf("The gRPC server is listening on port %d.", grpcPort)

	err = serveUntilShutdown(
		ctx,
		grpcServer.grpcServer,
		listener,
		shutdownChan,
		defaultGracefulShutdownTimeout,
	)
	if err != nil {
		tlog.E(ctx).Err(err).Msgf("The gRPC server terminated unexpectedly: %v.", err)
	}
}

func serveUntilShutdown(ctx context.Context, grpcServer grpcLifecycleServer, listener net.Listener, shutdownSignals <-chan os.Signal, shutdownTimeout time.Duration) error {
	serveErrChan := make(chan error, 1)
	go func() {
		serveErrChan <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serveErrChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		shutdownServer(shutdownCtx, grpcServer)
	case <-shutdownSignals:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		shutdownServer(shutdownCtx, grpcServer)
	}

	return <-serveErrChan
}

func shutdownServer(ctx context.Context, grpcServer interface {
	GracefulStop()
	Stop()
}) {
	if grpcServer == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	if ctx == nil {
		<-done
		return
	}

	select {
	case <-done:
		return
	case <-ctx.Done():
		grpcServer.Stop()
		<-done
	}
}
