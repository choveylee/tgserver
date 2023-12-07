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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func recoveryHandler(ctx context.Context, r interface{}) error {
	return status.Errorf(codes.Unknown, "recover from %v", r)
}
