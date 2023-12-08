/**
 * @Author: lidonglin
 * @Description:
 * @File:  metric.go
 * @Version: 1.0.0
 * @Date: 2023/12/08 09:54
 */

package tgserver

import (
	"context"

	"github.com/choveylee/tlog"
	"github.com/choveylee/tmetric"
)

var grpcServerLatency *tmetric.HistogramVec

func init() {
	var err error
	grpcServerLatency, err = tmetric.NewHistogramVec(
		"grpc_server_latency",
		"end to end latency",
		[]string{"type", "service", "method", "code"},
	)
	if err != nil {
		tlog.E(context.Background()).Err(err).Msgf("new http server metric err (%v).", err)
	}
}
