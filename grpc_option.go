package tgserver

import (
	"google.golang.org/grpc"
)

const defaultAccessLogPayloadLimit = 4096

type accessLogConfig struct {
	includeUnaryPayloads bool
	maxPayloadBytes      int
}

// GrpcOption stores service registration callbacks and additional [grpc.ServerOption]
// values used by StartGrpcServer.
type GrpcOption struct {
	registrars []func(grpc.ServiceRegistrar)

	options []grpc.ServerOption

	accessLog accessLogConfig
}

// NewGrpcOption returns an empty GrpcOption.
func NewGrpcOption() *GrpcOption {
	return &GrpcOption{
		registrars: make([]func(grpc.ServiceRegistrar), 0),

		options: make([]grpc.ServerOption, 0),
	}
}

// WithServiceRegistrar appends a service registration callback that is invoked before
// the server begins serving.
func (p *GrpcOption) WithServiceRegistrar(f func(grpc.ServiceRegistrar)) {
	p.registrars = append(p.registrars, f)
}

// WithServerOption appends a [grpc.ServerOption] to the server configuration.
func (p *GrpcOption) WithServerOption(option grpc.ServerOption) {
	p.options = append(p.options, option)
}

// WithUnaryAccessLogPayloads enables unary request and response payload logging for
// access logs. Serialized payloads larger than maxBytes are truncated before they are
// written. If maxBytes is not positive, a default limit is used.
func (p *GrpcOption) WithUnaryAccessLogPayloads(maxBytes int) {
	if maxBytes <= 0 {
		maxBytes = defaultAccessLogPayloadLimit
	}

	p.accessLog.includeUnaryPayloads = true
	p.accessLog.maxPayloadBytes = maxBytes
}

// WithGRPCRegister returns a helper that appends a service registration callback to a
// GrpcOption.
func WithGRPCRegister(register func(s grpc.ServiceRegistrar)) func(*GrpcOption) {
	return func(o *GrpcOption) {
		o.registrars = append(o.registrars, register)
	}
}
