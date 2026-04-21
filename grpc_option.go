package tgserver

import (
	"google.golang.org/grpc"
)

// GrpcOption groups service registrars and extra grpc.ServerOptions for StartGrpcServer.
type GrpcOption struct {
	registrars []func(grpc.ServiceRegistrar)

	options []grpc.ServerOption
}

// NewGrpcOption returns an empty GrpcOption.
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

// WithServerOption appends a grpc.ServerOption.
func (p *GrpcOption) WithServerOption(option grpc.ServerOption) {
	p.options = append(p.options, option)
}

// WithGRPCRegister returns an option function for functional-style service registration.
func WithGRPCRegister(register func(s grpc.ServiceRegistrar)) func(*GrpcOption) {
	return func(o *GrpcOption) {
		o.registrars = append(o.registrars, register)
	}
}
