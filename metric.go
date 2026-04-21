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
		tlog.E(context.Background()).Err(err).Msgf("new grpc server latency metric err (%v).", err)
	}
}
