package tgserver

import (
	"context"
	"fmt"

	"github.com/choveylee/tlog"
	"google.golang.org/grpc/grpclog"
)

type grpcLogger struct {
	I func() *tlog.Tevent
	W func() *tlog.Tevent
	E func() *tlog.Tevent
	F func() *tlog.Tevent

	verbosity int
}

func (l *grpcLogger) Info(args ...interface{}) {
	l.I().Msg(fmt.Sprint(args...))
}

func (l *grpcLogger) Infoln(args ...interface{}) {
	l.I().Msg(fmt.Sprintln(args...))
}

func (l *grpcLogger) Infof(format string, args ...interface{}) {
	l.I().Msgf(format, args...)
}

func (l *grpcLogger) Warning(args ...interface{}) {
	l.W().Msg(fmt.Sprint(args...))
}

func (l *grpcLogger) Warningln(args ...interface{}) {
	l.W().Msg(fmt.Sprintln(args...))
}

func (l *grpcLogger) Warningf(format string, args ...interface{}) {
	l.W().Msgf(format, args...)
}

func (l *grpcLogger) Error(args ...interface{}) {
	l.E().Msg(fmt.Sprint(args...))
}

func (l *grpcLogger) Errorln(args ...interface{}) {
	l.E().Msg(fmt.Sprintln(args...))
}

func (l *grpcLogger) Errorf(format string, args ...interface{}) {
	l.E().Msgf(format, args...)
}

func (l *grpcLogger) Fatal(args ...interface{}) {
	l.F().Msg(fmt.Sprint(args...))
}

func (l *grpcLogger) Fatalln(args ...interface{}) {
	l.F().Msg(fmt.Sprintln(args...))
}

func (l *grpcLogger) Fatalf(format string, args ...interface{}) {
	l.F().Msgf(format, args...)
}

func (l *grpcLogger) V(level int) bool {
	return level <= l.verbosity
}

// ReplaceGrpcLoggerV2 routes gRPC's process-global grpclog output through tlog.
// Call it during application initialization when you want that process-wide logging
// behavior.
func ReplaceGrpcLoggerV2() {
	grpcLogger := &grpcLogger{
		I: func() *tlog.Tevent {
			return tlog.I(context.Background())
		},
		W: func() *tlog.Tevent {
			return tlog.W(context.Background())
		},
		E: func() *tlog.Tevent {
			return tlog.E(context.Background())
		},
		F: func() *tlog.Tevent {
			return tlog.F(context.Background())
		},

		verbosity: 0,
	}

	grpclog.SetLoggerV2(grpcLogger)
}
