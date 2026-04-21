package tgserver

import (
	"context"
	"time"

	"google.golang.org/grpc/metadata"
)

type grpcCtx struct{}

type startTimeCtxKey struct{}

// WithValue attaches a mutable string-to-value map on ctx, alongside standard context values.
// Writes on the same chain update the same underlying map.
func WithValue(ctx context.Context, key string, val interface{}) context.Context {
	data, ok := ctx.Value(grpcCtx{}).(*map[string]interface{})
	if !ok {
		data = &map[string]interface{}{}

		ctx = context.WithValue(ctx, grpcCtx{}, data)
	}

	(*data)[key] = val

	return ctx
}

// GetValue returns a value set by WithValue; ok is false if the key is missing.
func GetValue(ctx context.Context, key string) (interface{}, bool) {
	data, ok := ctx.Value(grpcCtx{}).(*map[string]interface{})
	if !ok {
		return nil, false
	}

	val, ok := (*data)[key]
	if !ok {
		return nil, false
	}

	return val, true
}

// GetValueFromMetaData returns metadata values for key from incoming gRPC metadata.
func GetValueFromMetaData(ctx context.Context, key string) ([]string, bool) {
	data, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, false
	}

	v, ok := data[key]
	if !ok {
		return nil, false
	}

	return v, true
}

// GetStartTime returns the time recorded by SetStartTime, or the zero value if unset or wrong type.
func GetStartTime(ctx context.Context) time.Time {
	startTime, ok := ctx.Value(startTimeCtxKey{}).(time.Time)
	if !ok {
		return time.Time{}
	}

	return startTime
}

// SetStartTime stores the RPC start time in ctx for interceptors (e.g. latency measurement).
func SetStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, startTimeCtxKey{}, startTime)
}
