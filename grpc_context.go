package tgserver

import (
	"context"
	"time"

	"google.golang.org/grpc/metadata"
)

type grpcValueKey string
type grpcContextValue struct{ value any }
type startTimeCtxKey struct{}

// WithValue returns a derived context with a string-keyed value. A child may
// override the key independently of its siblings.
func WithValue(ctx context.Context, key string, value any) context.Context {
	stored := grpcContextValue{value: value}

	return context.WithValue(ctx, grpcValueKey(key), stored)
}

// GetValue returns the value associated with key, including an explicit nil.
func GetValue(ctx context.Context, key string) (any, bool) {
	stored, ok := ctx.Value(grpcValueKey(key)).(grpcContextValue)

	return stored.value, ok
}

// GetValueFromMetaData returns a copy of the matching incoming metadata values.
func GetValueFromMetaData(ctx context.Context, key string) ([]string, bool) {
	values := metadata.ValueFromIncomingContext(ctx, key)
	if len(values) == 0 {
		return nil, false
	}

	return values, true
}

// GetStartTime returns the start time recorded by SetStartTime or the zero time.
func GetStartTime(ctx context.Context) time.Time {
	startTime, _ := ctx.Value(startTimeCtxKey{}).(time.Time)

	return startTime
}

// SetStartTime stores the RPC start time in ctx for interceptor use.
func SetStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, startTimeCtxKey{}, startTime)
}
