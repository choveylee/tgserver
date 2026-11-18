package tgserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/choveylee/tlog"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

const defaultGracefulShutdownTimeout = 30 * time.Second

// ErrNoServiceRegistrars reports that StartGrpcServer was called without a registrar.
var ErrNoServiceRegistrars = errors.New("no gRPC service registrars configured")

// GrpcServer holds the gRPC server instance and the options used to construct it.
type GrpcServer struct {
	grpcOption GrpcOption

	grpcServer *grpc.Server
}

// StartGrpcServer serves until ctx is canceled, a shutdown signal arrives, or
// serving fails. Port zero lets the operating system choose an available port.
// If requests outlive the drain timeout, transport shutdown continues in the
// background and the returned error wraps context.DeadlineExceeded.
func StartGrpcServer(ctx context.Context, optionSet GrpcOption, port int) error {
	if ctx == nil {
		return errors.New("tgserver: context must not be nil")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("tgserver: start gRPC server: %w", err)
	}

	if port < 0 || port > 65535 {
		return fmt.Errorf("tgserver: gRPC port must be between 0 and 65535, got %d", port)
	}

	if optionSet.validationErr != nil {
		return optionSet.validationErr
	}

	if len(optionSet.registrars) == 0 {
		return ErrNoServiceRegistrars
	}

	serverCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	server, err := newGrpcServer(serverCtx, optionSet)
	if err != nil {
		return err
	}

	tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		server.Stop()

		return fmt.Errorf("tgserver: listen on gRPC port %d: %w", port, err)
	}

	if err := serverCtx.Err(); err != nil {
		_ = tcpListener.Close()
		server.Stop()

		return fmt.Errorf("tgserver: start gRPC server: %w", err)
	}

	listener := &shutdownListener{
		TCPListener:    tcpListener,
		connectionsMap: make(map[*shutdownConnection]struct{}),
	}

	tlog.I(ctx).Msgf("Start gRPC listener (address=%s)", listener.Addr())

	timeout := optionSet.shutdownTimeout
	if timeout == 0 {
		timeout = defaultGracefulShutdownTimeout
	}

	return serveUntilShutdown(serverCtx, server, listener, timeout)
}

// newGrpcServer keeps recovery and telemetry outside caller-supplied chained
// interceptors. gRPC rejects a second standalone interceptor during setup.
func newGrpcServer(ctx context.Context, optionSet GrpcOption) (_ *grpc.Server, err error) {
	var server *grpc.Server
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("tgserver: configure gRPC server: panic of type %T", value)
		}

		if err != nil && server != nil {
			server.Stop()
		}
	}()

	methodsMap := make(map[string]struct{})
	statsHandler := grpcServerStats{Handler: otelgrpc.NewServerHandler(), methodsMap: methodsMap}
	options := []grpc.ServerOption{
		grpc.StatsHandler(statsHandler),
		grpc.UnaryInterceptor(newUnaryServerInterceptor(optionSet.accessLog)),
		grpc.StreamInterceptor(streamServerInterceptor),
	}

	server = grpc.NewServer(append(options, optionSet.options...)...)
	registry := &serviceRegistry{server: server}
	defer registry.closeRegistration()

	for _, registrar := range optionSet.registrars {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("tgserver: register gRPC services: %w", err)
		}

		registrar(registry)

		registry.mu.Lock()
		registrationErr := registry.err
		registry.mu.Unlock()
		if registrationErr != nil {
			return nil, registrationErr
		}
	}

	if err := registry.closeRegistration(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("tgserver: register gRPC services: %w", err)
	}

	// After registration closes, the telemetry handlers only read this map.
	for service, serviceInfo := range server.GetServiceInfo() {
		for _, method := range serviceInfo.Methods {
			methodsMap["/"+service+"/"+method.Name] = struct{}{}
		}
	}

	return server, nil
}

// serveUntilShutdown waits for a successful drain. After the deadline, running
// handlers might ignore cancellation or block grpc.Stop itself.
func serveUntilShutdown(ctx context.Context, server *grpc.Server, listener *shutdownListener, timeout time.Duration) error {
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()

	select {
	case err := <-serveResult:
		if err != nil {
			listener.forceClose()
			go server.Stop()

			return fmt.Errorf("tgserver: serve gRPC listener: %w", err)
		}

		return nil
	case <-ctx.Done():
	}

	drained := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(drained)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-drained:
		err := <-serveResult
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("tgserver: serve gRPC listener: %w", err)
		}

		return nil
	case <-timer.C:
		listener.forceClose()
		go server.Stop()

		return fmt.Errorf("tgserver: drain gRPC server: %w", context.DeadlineExceeded)
	}
}

// serviceRegistry validates registrations that the upstream registrar would
// otherwise handle by terminating the process. Registration runs before Serve.
type serviceRegistry struct {
	server *grpc.Server

	mu     sync.Mutex
	closed bool
	err    error
}

// RegisterService rejects invalid or duplicate registrations before gRPC can
// terminate the process. Calls made after the registration phase have no effect.
func (registry *serviceRegistry) RegisterService(description *grpc.ServiceDesc, implementation any) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if registry.closed || registry.err != nil {
		return
	}

	if description == nil || description.ServiceName == "" {
		registry.err = errors.New("tgserver: gRPC service description and name must be set")

		return
	}

	if _, ok := registry.server.GetServiceInfo()[description.ServiceName]; ok {
		registry.err = fmt.Errorf("tgserver: gRPC service %q is already registered", description.ServiceName)

		return
	}

	if description.HandlerType != nil || implementation != nil {
		handlerType := reflect.TypeOf(description.HandlerType)
		implementationType := reflect.TypeOf(implementation)
		if handlerType == nil || handlerType.Kind() != reflect.Pointer || handlerType.Elem().Kind() != reflect.Interface ||
			isNilValue(implementation) || !implementationType.Implements(handlerType.Elem()) {
			registry.err = fmt.Errorf("tgserver: invalid implementation for gRPC service %q", description.ServiceName)

			return
		}
	}

	// gRPC retains pointers to individual methods and streams.
	registered := *description
	registered.Methods = slices.Clone(description.Methods)
	registered.Streams = slices.Clone(description.Streams)
	if err := validateServiceMethods(&registered); err != nil {
		registry.err = err

		return
	}

	registry.server.RegisterService(&registered, implementation)
}

// closeRegistration seals the registrar before the server can enter Serve.
func (registry *serviceRegistry) closeRegistration() error {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.closed = true

	return registry.err
}

// validateServiceMethods catches nil and duplicate descriptors before gRPC
// builds a method table that can overwrite handlers or call a nil function.
func validateServiceMethods(description *grpc.ServiceDesc) error {
	methodsMap := make(map[string]struct{}, len(description.Methods)+len(description.Streams))
	record := func(name string, hasHandler bool) error {
		if name == "" || !hasHandler {
			return fmt.Errorf("tgserver: method %q in gRPC service %q must have a name and handler", name, description.ServiceName)
		}

		if _, ok := methodsMap[name]; ok {
			return fmt.Errorf("tgserver: method %q in gRPC service %q is already registered", name, description.ServiceName)
		}

		methodsMap[name] = struct{}{}

		return nil
	}

	for _, method := range description.Methods {
		if err := record(method.MethodName, method.Handler != nil); err != nil {
			return err
		}
	}

	for _, stream := range description.Streams {
		if err := record(stream.StreamName, stream.Handler != nil); err != nil {
			return err
		}
	}

	return nil
}

// GetServiceInfo makes the startup registrar compatible with gRPC reflection.
func (registry *serviceRegistry) GetServiceInfo() map[string]grpc.ServiceInfo {
	return registry.server.GetServiceInfo()
}
