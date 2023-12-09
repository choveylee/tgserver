/**
 * @Author: lidonglin
 * @Description:
 * @File:  log.go
 * @Version: 1.0.0
 * @Date: 2023/12/08 11:05
 */

package tgserver

import (
	"context"
	"time"

	"github.com/choveylee/tlog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func logFormatter(ctx context.Context, service, method string, req interface{}, resp interface{}, err error) {
	code := status.Code(err)

	latency := time.Since(GetStartTime(ctx))

	var event *tlog.Tevent

	if code == codes.OK {
		event = tlog.D(ctx)
	} else {
		event = tlog.E(ctx).Err(err)
	}

	event = event.Detailf("service:%s", service).
		Detailf("method:%s", method).
		Detailf("latency:%v", latency).
		Detailf("code:%s", code.String())

	reqData := make([]byte, 0)
	respData := make([]byte, 0)

	request, ok := req.(proto.Message)
	if ok {
		var err error

		reqData, err = protojson.Marshal(request)
		if err != nil {
			tlog.W(ctx).Err(err).Msgf("marshal req proto message err (%v).", err)
		}
	}

	response, ok := resp.(proto.Message)
	if ok {
		var err error

		respData, err = protojson.Marshal(response)
		if err != nil {
			tlog.W(ctx).Err(err).Msgf("marshal resp proto message err (%v).", err)
		}
	}

	event = event.Detailf("req:%s", string(reqData)).
		Detailf("resp:%s", string(respData))

	event.Msg("grpc access log")
}
