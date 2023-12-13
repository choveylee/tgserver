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
	"runtime"
	"strings"
	"time"

	"github.com/choveylee/tlog"
	"github.com/choveylee/tmetric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	Unary        = "unary"
	ClientStream = "client_stream"
	ServerStream = "server_stream"
	BidiStream   = "bidi_stream"
)

func funcFileLine(excludePKG string) (string, string, int) {
	const depth = 8
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	ff := runtime.CallersFrames(pcs[:n])

	var fn, file string
	var line int
	for {
		f, ok := ff.Next()
		if !ok {
			break
		}

		fn, file, line = f.Function, f.File, f.Line
		if !strings.Contains(fn, excludePKG) {
			break
		}
	}

	if index := strings.LastIndexByte(fn, '/'); index != -1 {
		fn = fn[index+1:]
	}

	return fn, file, line
}

func recoveryHandler(ctx context.Context, r interface{}) error {
	_, file, line := funcFileLine("github.com/choveylee")

	errMsg := tlog.E(ctx).Msgf("recover from panic (%s, %d, %v).",
		file, line, r)

	return status.Error(codes.Unknown, errMsg)
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

	code := status.Code(err)

	latency := time.Since(GetStartTime(ctx))

	var event *tlog.Tevent

	if code == codes.OK {
		event = tlog.D(ctx)
	} else {
		event = tlog.E(ctx).Err(err)
	}

	event = event.Detailf("service:%s", service).
		Detailf("method:%s", method).
		Detailf("latency:%v", latency).
		Detailf("code:%s", code.String())

	reqData := make([]byte, 0)
	respData := make([]byte, 0)

	request, ok := req.(proto.Message)
	if ok {
		var err error

		reqData, err = protojson.Marshal(request)
		if err != nil {
			tlog.W(ctx).Err(err).Msgf("marshal req proto message err (%v).", err)
		}
	}

	response, ok := resp.(proto.Message)
	if ok {
		var err error

		respData, err = protojson.Marshal(response)
		if err != nil {
			tlog.W(ctx).Err(err).Msgf("marshal resp proto message err (%v).", err)
		}
	}

	event = event.Detailf("req:%s", string(reqData)).
		Detailf("resp:%s", string(respData))

	event.Msg("grpc access log")

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
