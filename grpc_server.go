package tgserver

import (
	"context"
	"errors"
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

// ErrNoServiceRegistrars reports that StartGrpcServer was called without any service registrars.
var ErrNoServiceRegistrars = errors.New("no gRPC service registrars configured")

type grpcLifecycleServer interface {
	Serve(net.Listener) error
	GracefulStop()
	Stop()
}

// GrpcServer holds the gRPC server instance and the options used to construct it.
type GrpcServer struct {
	grpcOption GrpcOption

	grpcServer *grpc.Server
}

func defaultServerOptions(grpcOption GrpcOption) []grpc.ServerOption {
	recoveryOptions := grpcrecovery.WithRecoveryHandlerContext(recoveryHandler)

	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			latencyServerInterceptor,
			prometheusServerInterceptor,
			newLogServerInterceptor(grpcOption.accessLog),
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
// Serve returns an unexpected error, or the process receives SIGINT or SIGTERM. During
// shutdown, it first attempts a graceful drain and then forces Stop if the drain does not
// complete within the configured timeout. Signal registration is released with [signal.Stop]
// before the function returns so repeated invocations do not accumulate handlers.
//
// StartGrpcServer returns a non-nil error if no service registrars are configured, if the
// listener cannot be created, or if Serve terminates unexpectedly.
func StartGrpcServer(ctx context.Context, grpcOption GrpcOption, grpcPort int) error {
	grpcServer := &GrpcServer{
		grpcOption: grpcOption,
	}

	if len(grpcServer.grpcOption.registrars) == 0 {
		tlog.E(ctx).Msg("gRPC server startup aborted because no service registrars are configured.")
		return ErrNoServiceRegistrars
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		tlog.E(ctx).Err(err).Msgf("Failed to create the gRPC listener on port %d.", grpcPort)
		return fmt.Errorf("listen on gRPC port %d: %w", grpcPort, err)
	}

	options := defaultServerOptions(grpcOption)
	options = append(options, grpcServer.grpcOption.options...)

	grpcServer.grpcServer = grpc.NewServer(options...)

	for _, registrar := range grpcServer.grpcOption.registrars {
		registrar(grpcServer.grpcServer)
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownChan)

	tlog.I(ctx).Msgf("gRPC server listening on %s.", listenerAddressForLog(listener))

	err = serveUntilShutdown(
		ctx,
		grpcServer.grpcServer,
		listener,
		shutdownChan,
		defaultGracefulShutdownTimeout,
	)
	if err != nil {
		tlog.E(ctx).Err(err).Msg("gRPC server terminated unexpectedly.")
		return err
	}

	return nil
}

func listenerAddressForLog(listener net.Listener) string {
	if listener == nil || listener.Addr() == nil {
		return "<unknown>"
	}

	return listener.Addr().String()
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
