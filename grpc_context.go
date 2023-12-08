/**
 * @Author: lidonglin
 * @Description:
 * @File:  grpc_context.go
 * @Version: 1.0.0
 * @Date: 2023/12/08 00:30
 */

package tgserver

import (
	"context"
	"time"

	"google.golang.org/grpc/metadata"
)

type grpcCtx struct{}

func WithValue(ctx context.Context, key string, val interface{}) context.Context {
	data, ok := ctx.Value(grpcCtx{}).(*map[string]interface{})
	if !ok {
		data = &map[string]interface{}{}

		ctx = context.WithValue(ctx, grpcCtx{}, data)
	}

	(*data)[key] = val

	return ctx
}

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

func GetStartTime(ctx context.Context) time.Time {
	value, ok := GetValue(ctx, "start_time")
	if !ok {
		return time.Time{}
	}

	return value.(time.Time)
}

func SetStartTime(ctx context.Context, startTime time.Time) context.Context {
	return WithValue(ctx, "start_time", startTime)
}
