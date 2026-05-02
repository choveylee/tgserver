package tgserver

import (
	"context"
	"time"

	"google.golang.org/grpc/metadata"
)

type grpcCtx struct{}

type grpcContextValues map[string]interface{}

type startTimeCtxKey struct{}

// WithValue returns a derived context that carries a string-keyed value map.
// Each call copies the existing map so that sibling contexts do not share mutable state.
func WithValue(ctx context.Context, key string, val interface{}) context.Context {
	data, _ := ctx.Value(grpcCtx{}).(grpcContextValues)
	copied := make(grpcContextValues, len(data)+1)
	for k, v := range data {
		copied[k] = v
	}
	copied[key] = val

	return context.WithValue(ctx, grpcCtx{}, copied)
}

// GetValue returns the value associated with key, if present.
func GetValue(ctx context.Context, key string) (interface{}, bool) {
	data, ok := ctx.Value(grpcCtx{}).(grpcContextValues)
	if !ok {
		return nil, false
	}

	val, ok := data[key]
	if !ok {
		return nil, false
	}

	return val, true
}

// GetValueFromMetaData returns the incoming gRPC metadata values associated with key.
// Lookup is case-insensitive, consistent with [metadata.MD.Get].
func GetValueFromMetaData(ctx context.Context, key string) ([]string, bool) {
	data, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, false
	}

	v := data.Get(key)
	if len(v) == 0 {
		return nil, false
	}

	return v, true
}

// GetStartTime returns the start time recorded by SetStartTime.
// It returns the zero value if no start time has been recorded.
func GetStartTime(ctx context.Context) time.Time {
	startTime, ok := ctx.Value(startTimeCtxKey{}).(time.Time)
	if !ok {
		return time.Time{}
	}

	return startTime
}

// SetStartTime stores the RPC start time in ctx for interceptor use, such as latency
// measurement.
func SetStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, startTimeCtxKey{}, startTime)
}
