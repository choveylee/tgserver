package tgserver

import (
	"context"
	"time"

	"github.com/choveylee/tlog"
	"github.com/choveylee/tmetric"
)

var grpcServerLatency *tmetric.HistogramVec

func init() {
	var err error
	grpcServerLatency, err = tmetric.NewHistogramVec(
		"grpc_server_latency",
		"End-to-end RPC latency in milliseconds",
		[]string{"type", "service", "method", "code"},
	)
	if err != nil {
		tlog.E(context.Background()).Err(err).Msg("Initialize gRPC server latency metric error")
	}
}

// observeRPC records the elapsed RPC duration before access-log serialization.
func observeRPC(ctx context.Context, callType, service, method, code string, duration time.Duration) {
	if grpcServerLatency == nil {
		return
	}

	if err := grpcServerLatency.Observe(float64(duration)/float64(time.Millisecond), callType, service, method, code); err != nil {
		tlog.W(ctx).Err(err).Msg("Record gRPC server latency metric error")
	}
}
