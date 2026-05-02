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

// RPC call-shape labels used in metrics and structured logs.
const (
	Unary        = "unary"
	ClientStream = "client_stream"
	ServerStream = "server_stream"
	BidiStream   = "bidi_stream"

	internalServerErrorMessage = "internal server error"
)

type contextServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (p *contextServerStream) Context() context.Context {
	return p.ctx
}

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

	tlog.E(ctx).Msgf("Recovered from panic at %s:%d: %v.",
		file, line, r)

	return status.Error(codes.Unknown, internalServerErrorMessage)
}

func splitMethodName(fullMethod string) (string, string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/") // remove leading slash

	index := strings.Index(fullMethod, "/")
	if index >= 0 {
		return fullMethod[:index], fullMethod[index+1:]
	}

	return "unknown", "unknown"
}

func streamRPCType(info *grpc.StreamServerInfo) string {
	switch {
	case info.IsClientStream && info.IsServerStream:
		return BidiStream
	case info.IsClientStream:
		return ClientStream
	case info.IsServerStream:
		return ServerStream
	default:
		return Unary
	}
}

func requestLatency(ctx context.Context) time.Duration {
	startTime := GetStartTime(ctx)
	if startTime.IsZero() {
		return 0
	}

	return time.Since(startTime)
}

func observeRPC(ctx context.Context, callType, fullMethod string, err error) {
	startTime := GetStartTime(ctx)
	if startTime.IsZero() || grpcServerLatency == nil {
		return
	}

	service, method := splitMethodName(fullMethod)
	grpcServerLatency.Observe(
		tmetric.SinceMS(startTime),
		callType,
		service,
		method,
		status.Code(err).String(),
	)
}

func marshalProtoMessage(ctx context.Context, msg interface{}, name string) []byte {
	protoMessage, ok := msg.(proto.Message)
	if !ok {
		return nil
	}

	data, err := protojson.Marshal(protoMessage)
	if err != nil {
		tlog.W(ctx).Err(err).Msgf("Failed to marshal the %s protobuf message: %v.", name, err)
		return nil
	}

	return data
}

func logRPC(ctx context.Context, callType, fullMethod string, err error, reqData, respData []byte) {
	service, method := splitMethodName(fullMethod)
	code := status.Code(err)
	latency := requestLatency(ctx)

	var event *tlog.Tevent
	if code == codes.OK {
		event = tlog.I(ctx)
	} else {
		event = tlog.E(ctx).Err(err)
	}

	event = event.Detailf("type:%s", callType).
		Detailf("service:%s", service).
		Detailf("method:%s", method).
		Detailf("latency:%v", latency).
		Detailf("code:%s", code.String())

	if reqData != nil {
		event = event.Detailf("req:%s", string(reqData))
	}
	if respData != nil {
		event = event.Detailf("resp:%s", string(respData))
	}

	event.Msg("gRPC request completed.")
}

func normalizeServerError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := status.FromError(err); ok {
		return err
	}

	return status.Error(codes.Unknown, err.Error())
}

func latencyServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return handler(SetStartTime(ctx, time.Now()), req)
}

func latencyStreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := SetStartTime(stream.Context(), time.Now())
	return handler(srv, &contextServerStream{
		ServerStream: stream,
		ctx:          ctx,
	})
}

func logServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	logRPC(
		ctx,
		Unary,
		info.FullMethod,
		err,
		marshalProtoMessage(ctx, req, "req"),
		marshalProtoMessage(ctx, resp, "resp"),
	)

	return resp, err
}

func logStreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(srv, stream)
	logRPC(stream.Context(), streamRPCType(info), info.FullMethod, err, nil, nil)
	return err
}

func prometheusServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	observeRPC(ctx, Unary, info.FullMethod, err)
	return resp, err
}

func prometheusStreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(srv, stream)
	observeRPC(stream.Context(), streamRPCType(info), info.FullMethod, err)
	return err
}

func errorServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	return resp, normalizeServerError(err)
}

func errorStreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return normalizeServerError(handler(srv, stream))
}
