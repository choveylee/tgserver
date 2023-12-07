/**
 * @Author: lidonglin
 * @Description:
 * @File:  grpc_option.go
 * @Version: 1.0.0
 * @Date: 2023/12/07 20:58
 */

package tgserver

import (
	"google.golang.org/grpc"
)

type GrpcOption struct {
	registrars []func(grpc.ServiceRegistrar)

	options []grpc.ServerOption
}

func NewGrpcOption() *GrpcOption {
	return &GrpcOption{
		registrars: make([]func(grpc.ServiceRegistrar), 0),

		options: make([]grpc.ServerOption, 0),
	}
}

func (p *GrpcOption) WithServiceRegistrar(f func(grpc.ServiceRegistrar)) {
	p.registrars = append(p.registrars, f)
}

func (p *GrpcOption) WithServerOption(option grpc.ServerOption) {
	p.options = append(p.options, option)
}

func WithGRPCRegister(register func(s grpc.ServiceRegistrar)) func(*GrpcOption) {
	return func(o *GrpcOption) {
		o.registrars = append(o.registrars, register)
	}
}
