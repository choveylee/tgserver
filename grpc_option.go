package tgserver

import (
	"errors"
	"reflect"
	"slices"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const defaultAccessLogPayloadLimit = 4096

type accessLogConfig struct {
	maxPayloadBytes int
}

// GrpcOption stores service registration callbacks and server options.
// Copies can be extended independently.
type GrpcOption struct {
	registrars []func(grpc.ServiceRegistrar)
	options    []grpc.ServerOption

	accessLog       accessLogConfig
	shutdownTimeout time.Duration
	validationErr   error
}

// NewGrpcOption returns an empty GrpcOption.
func NewGrpcOption() *GrpcOption {
	return new(GrpcOption)
}

// WithServiceRegistrar appends a callback invoked synchronously before serving.
func (optionSet *GrpcOption) WithServiceRegistrar(registrar func(grpc.ServiceRegistrar)) {
	if registrar == nil {
		optionSet.validationErr = errors.New("tgserver: gRPC service registrar must not be nil")

		return
	}

	optionSet.registrars = append(slices.Clip(optionSet.registrars), registrar)
}

// WithServerOption appends a gRPC option. Add custom interceptors with
// [grpc.ChainUnaryInterceptor] or [grpc.ChainStreamInterceptor].
func (optionSet *GrpcOption) WithServerOption(option grpc.ServerOption) {
	if isNilValue(option) {
		optionSet.validationErr = errors.New("tgserver: gRPC server option must not be nil")

		return
	}

	optionSet.options = append(slices.Clip(optionSet.options), option)
}

// WithTransportCredentials installs credentials and releases socket ownership
// when the handshake returns [credentials.ErrConnDispatched].
func (optionSet *GrpcOption) WithTransportCredentials(transportCredentials credentials.TransportCredentials) {
	if isNilValue(transportCredentials) {
		optionSet.validationErr = errors.New("tgserver: gRPC transport credentials must not be nil")

		return
	}

	tracked := trackedCredentials{TransportCredentials: transportCredentials}

	optionSet.WithServerOption(grpc.Creds(tracked))
}

// WithUnaryAccessLogPayloads enables bounded, redacted unary protobuf diagnostics.
// A nonpositive limit uses the default.
func (optionSet *GrpcOption) WithUnaryAccessLogPayloads(maxBytes int) {
	if maxBytes <= 0 {
		maxBytes = defaultAccessLogPayloadLimit
	}

	optionSet.accessLog.maxPayloadBytes = maxBytes
}

// WithGracefulShutdownTimeout sets how long the server waits for RPCs to drain.
func (optionSet *GrpcOption) WithGracefulShutdownTimeout(timeout time.Duration) {
	if timeout <= 0 {
		optionSet.validationErr = errors.New("tgserver: gRPC shutdown timeout must be positive")

		return
	}

	optionSet.shutdownTimeout = timeout
}

// WithGRPCRegister returns a helper that appends a service registrar.
func WithGRPCRegister(registrar func(grpc.ServiceRegistrar)) func(*GrpcOption) {
	return func(optionSet *GrpcOption) {
		optionSet.WithServiceRegistrar(registrar)
	}
}

// isNilValue detects a nil value even when an interface carries its type.
func isNilValue(value any) bool {
	reflectedValue := reflect.ValueOf(value)
	switch reflectedValue.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflectedValue.IsNil()
	}

	return false
}
