package tgserver

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	"github.com/choveylee/tlog"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// RPC shape labels used in metrics and structured logs.
const (
	Unary        = "unary"
	ClientStream = "client_stream"
	ServerStream = "server_stream"
	BidiStream   = "bidi_stream"

	internalServerErrorMessage = "internal server error"
	unknownRPCMethod           = "/unknown/unknown"
)

type contextServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (stream *contextServerStream) Context() context.Context {
	return stream.ctx
}

// newUnaryServerInterceptor snapshots opt-in request diagnostics before the
// handler can mutate the request, and recovers panics from the handler chain.
func newUnaryServerInterceptor(config accessLogConfig) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				response = nil
				err = recoverServerPanic(ctx, recovered)
			}
		}()

		ctx = prepareRPCContext(ctx)
		observation, _ := ctx.Value(rpcObservationKey{}).(*rpcObservation)
		if observation != nil && config.maxPayloadBytes > 0 {
			observation.requestPayload = marshalProtoMessage(ctx, request, config.maxPayloadBytes)
		}

		response, err = handler(ctx, request)
		if err == nil && observation != nil && config.maxPayloadBytes > 0 {
			observation.responsePayload = marshalProtoMessage(ctx, response, config.maxPayloadBytes)
		}

		return response, normalizeServerError(err)
	}
}

// streamServerInterceptor preserves the stream context and recovers panics from
// the handler chain.
func streamServerInterceptor(service any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	ctx := prepareRPCContext(stream.Context())
	streamWithContext := &contextServerStream{ServerStream: stream, ctx: ctx}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = recoverServerPanic(ctx, recovered)
		}
	}()

	return normalizeServerError(handler(service, streamWithContext))
}

// prepareRPCContext attaches a deferred span while keeping gRPC transport
// values and caller-supplied context available to the handler.
func prepareRPCContext(ctx context.Context) context.Context {
	observation, _ := ctx.Value(rpcObservationKey{}).(*rpcObservation)
	if observation != nil && observation.telemetryCtx != nil {
		ctx = trace.ContextWithSpan(ctx, trace.SpanFromContext(observation.telemetryCtx))
		ctx = baggage.ContextWithBaggage(ctx, baggage.FromContext(observation.telemetryCtx))
	}

	return SetStartTime(ctx, time.Now())
}

// normalizeServerError preserves the explicit public gRPC status and its
// details, and discards private context added by wrappers.
func normalizeServerError(err error) error {
	if err == nil {
		return nil
	}

	var statusError interface{ GRPCStatus() *status.Status }
	if errors.As(err, &statusError) {
		if publicStatus := statusError.GRPCStatus(); publicStatus != nil {
			return publicStatus.Err()
		}
	}

	switch {
	case errors.Is(err, context.Canceled):
		return status.FromContextError(context.Canceled).Err()
	case errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(context.DeadlineExceeded).Err()
	default:
		return status.Error(codes.Unknown, internalServerErrorMessage)
	}
}

// recoverServerPanic logs the panic type and its frame without serializing the value.
func recoverServerPanic(ctx context.Context, recovered any) error {
	var callers [16]uintptr
	count := runtime.Callers(3, callers[:])
	frames := runtime.CallersFrames(callers[:count])
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "runtime.") || !more {
			tlog.E(ctx).
				Detailf("panic type:%T", recovered).
				Msgf("Recover gRPC handler panic (file=%s, line=%d)", frame.File, frame.Line)

			break
		}
	}

	return status.Error(codes.Unknown, internalServerErrorMessage)
}

type rpcObservationKey struct{}

type rpcObservation struct {
	service string
	method  string
	rpcType string

	unknownSince time.Time
	telemetryCtx context.Context

	requestPayload  string
	responsePayload string
}

// logRPC uses the final transport status and the same elapsed time for logs and metrics.
func logRPC(ctx context.Context, observation *rpcObservation, code codes.Code, duration time.Duration) {
	observeRPC(ctx, observation.rpcType, observation.service, observation.method, code.String(), duration)

	var event *tlog.Tevent
	if code == codes.OK {
		event = tlog.I(ctx)
	} else {
		event = tlog.E(ctx)
	}

	event.Detailf("type:%s", observation.rpcType).
		Detailf("service:%s", observation.service).
		Detailf("method:%s", observation.method).
		Detailf("latency:%v", duration).
		Detailf("code:%s", code.String())

	if observation.requestPayload != "" {
		event.Detailf("req:%s", observation.requestPayload)
	}

	if code == codes.OK && observation.responsePayload != "" {
		event.Detailf("resp:%s", observation.responsePayload)
	}

	event.Msg("Complete gRPC request")
}

// streamRPCType maps the method shape reported by gRPC to a telemetry label.
func streamRPCType(clientStream, serverStream bool) string {
	switch {
	case clientStream && serverStream:
		return BidiStream
	case clientStream:
		return ClientStream
	case serverStream:
		return ServerStream
	default:
		return Unary
	}
}

type grpcServerStats struct {
	stats.Handler

	methodsMap map[string]struct{}
}

// TagRPC bounds telemetry labels to registered methods and preserves the raw
// path for the transport and caller-supplied stats handlers.
func (handler grpcServerStats) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	service, method, valid := splitMethodName(info.FullMethodName)
	observation := &rpcObservation{service: service, method: method, rpcType: Unary}
	if _, registered := handler.methodsMap[info.FullMethodName]; !registered || !valid {
		// A canceled, unhandled RPC may emit neither Begin nor OutTrailer.
		observation.service, observation.method = "unknown", "unknown"
		observation.unknownSince = time.Now()
	} else {
		ctx = handler.Handler.TagRPC(ctx, info)
	}

	return context.WithValue(ctx, rpcObservationKey{}, observation)
}

// HandleRPC ends the built-in span with sanitized status and records access
// telemetry after gRPC has encoded and attempted to send the response.
func (handler grpcServerStats) HandleRPC(ctx context.Context, event stats.RPCStats) {
	observation, _ := ctx.Value(rpcObservationKey{}).(*rpcObservation)
	if observation != nil && !observation.unknownSince.IsZero() {
		if observation.telemetryCtx == nil {
			switch event.(type) {
			case *stats.Begin:
				observation.telemetryCtx = handler.Handler.TagRPC(ctx, &stats.RPCTagInfo{FullMethodName: unknownRPCMethod})
			case *stats.OutTrailer:
				handler.recordUnknownRejection(ctx, observation)

				return
			default:
				return
			}
		}

		ctx = observation.telemetryCtx
	}

	switch rpcEvent := event.(type) {
	case *stats.Begin:
		if observation != nil {
			observation.rpcType = streamRPCType(rpcEvent.IsClientStream, rpcEvent.IsServerStream)
		}
	case *stats.End:
		if rpcEvent.Error != nil {
			sanitizedEnd := *rpcEvent
			sanitizedEnd.Error = status.Error(status.Code(rpcEvent.Error), "")

			event = &sanitizedEnd
		}
	}

	handler.Handler.HandleRPC(ctx, event)

	if end, ok := event.(*stats.End); ok && observation != nil {
		logRPC(ctx, observation, status.Code(end.Error), end.EndTime.Sub(end.BeginTime))
	}
}

// recordUnknownRejection completes telemetry for the native gRPC rejection,
// which emits OutTrailer without a Begin or End event.
func (handler grpcServerStats) recordUnknownRejection(ctx context.Context, observation *rpcObservation) {
	end := &stats.End{
		BeginTime: observation.unknownSince,
		EndTime:   time.Now(),
		Error:     status.Error(codes.Unimplemented, ""),
	}
	ctx = handler.Handler.TagRPC(ctx, &stats.RPCTagInfo{FullMethodName: unknownRPCMethod})
	handler.Handler.HandleRPC(ctx, end)
	logRPC(ctx, observation, codes.Unimplemented, end.EndTime.Sub(end.BeginTime))
}

// splitMethodName recognizes the protobuf RPC path used in telemetry.
func splitMethodName(fullMethod string) (string, string, bool) {
	path, prefixed := strings.CutPrefix(fullMethod, "/")
	service, method, separated := strings.Cut(path, "/")
	valid := prefixed && separated && protoreflect.FullName(service).IsValid() && protoreflect.Name(method).IsValid()

	return service, method, valid
}
