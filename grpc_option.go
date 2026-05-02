package tgserver

import (
	"google.golang.org/grpc"
)

// GrpcOption stores service registrars and additional [grpc.ServerOption] values for StartGrpcServer.
type GrpcOption struct {
	registrars []func(grpc.ServiceRegistrar)

	options []grpc.ServerOption
}

// NewGrpcOption constructs an empty GrpcOption.
func NewGrpcOption() *GrpcOption {
	return &GrpcOption{
		registrars: make([]func(grpc.ServiceRegistrar), 0),

		options: make([]grpc.ServerOption, 0),
	}
}

// WithServiceRegistrar appends a service registration callback.
func (p *GrpcOption) WithServiceRegistrar(f func(grpc.ServiceRegistrar)) {
	p.registrars = append(p.registrars, f)
}

// WithServerOption appends an additional [grpc.ServerOption].
func (p *GrpcOption) WithServerOption(option grpc.ServerOption) {
	p.options = append(p.options, option)
}

// WithGRPCRegister returns a helper that applies a service registration callback to a GrpcOption.
func WithGRPCRegister(register func(s grpc.ServiceRegistrar)) func(*GrpcOption) {
	return func(o *GrpcOption) {
		o.registrars = append(o.registrars, register)
	}
}
