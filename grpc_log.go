package tgserver

import (
	"context"
	"fmt"

	"github.com/choveylee/tlog"
	"google.golang.org/grpc/grpclog"
)

type grpcLogger struct{}

func (grpcLogger) Info(arguments ...any) {
	tlog.I(context.Background()).Msg(fmt.Sprint(arguments...))
}

func (grpcLogger) Infoln(arguments ...any) {
	tlog.I(context.Background()).Msg(fmt.Sprintln(arguments...))
}

func (grpcLogger) Infof(format string, arguments ...any) {
	tlog.I(context.Background()).Msgf(format, arguments...)
}

func (grpcLogger) Warning(arguments ...any) {
	tlog.W(context.Background()).Msg(fmt.Sprint(arguments...))
}

func (grpcLogger) Warningln(arguments ...any) {
	tlog.W(context.Background()).Msg(fmt.Sprintln(arguments...))
}

func (grpcLogger) Warningf(format string, arguments ...any) {
	tlog.W(context.Background()).Msgf(format, arguments...)
}

func (grpcLogger) Error(arguments ...any) {
	tlog.E(context.Background()).Msg(fmt.Sprint(arguments...))
}

func (grpcLogger) Errorln(arguments ...any) {
	tlog.E(context.Background()).Msg(fmt.Sprintln(arguments...))
}

func (grpcLogger) Errorf(format string, arguments ...any) {
	tlog.E(context.Background()).Msgf(format, arguments...)
}

func (grpcLogger) Fatal(arguments ...any) {
	tlog.F(context.Background()).Msg(fmt.Sprint(arguments...))
}

func (grpcLogger) Fatalln(arguments ...any) {
	tlog.F(context.Background()).Msg(fmt.Sprintln(arguments...))
}

func (grpcLogger) Fatalf(format string, arguments ...any) {
	tlog.F(context.Background()).Msgf(format, arguments...)
}

func (grpcLogger) V(level int) bool {
	return level <= 0
}

// ReplaceGrpcLoggerV2 routes the process-wide gRPC logger through tlog.
// Call it during application initialization.
func ReplaceGrpcLoggerV2() {
	grpclog.SetLoggerV2(grpcLogger{})
}
