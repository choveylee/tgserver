/**
 * @Author: lidonglin
 * @Description:
 * @File:  grpc_interceptor.go
 * @Version: 1.0.0
 * @Date: 2023/12/07 23:28
 */

package tgserver

import (
	"context"
	"strings"
	"time"

	"github.com/choveylee/tmetric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	Unary        = "unary"
	ClientStream = "client_stream"
	ServerStream = "server_stream"
	BidiStream   = "bidi_stream"
)

func recoveryHandler(ctx context.Context, r interface{}) error {
	return status.Errorf(codes.Unknown, "recover from %v", r)
}

func splitMethodName(fullMethod string) (string, string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/") // remove leading slash

	index := strings.Index(fullMethod, "/")
	if index >= 0 {
		return fullMethod[:index], fullMethod[index+1:]
	}

	return "unknown", "unknown"
}

func latencyServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return handler(SetStartTime(ctx, time.Now()), req)
}

func logServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)

	service, method := splitMethodName(info.FullMethod)

	logFormatter(ctx, service, method, req, resp, err)

	return resp, err
}

func prometheusServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)

	service, method := splitMethodName(info.FullMethod)

	startTime := GetStartTime(ctx)

	grpcServerLatency.Observe(tmetric.SinceMS(startTime), string(Unary), service, method, status.Code(err).String())

	return resp, err
}

func errorServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)

	_, ok := status.FromError(err)
	if ok {
		return resp, err
	}

	// TODO

	return resp, err
}
