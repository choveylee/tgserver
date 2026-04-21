# tgserver

`tgserver` is a small Go library for starting a production-oriented [gRPC](https://grpc.io/) server with common operational plumbing: **OpenTelemetry** server metrics, **Prometheus** latency histograms, structured **access logging** (via [tlog](https://github.com/choveylee/tlog)), **panic recovery**, and helpers for **request-scoped context** values.

## Requirements

- **Go** 1.25 or later (see `go.mod`).

## Installation

```bash
go get github.com/choveylee/tgserver
```

## Quick start

Register your gRPC services, then call `StartGrpcServer`. The server listens on `tcp` on the given port, serves until the context is cancelled or the process receives **SIGINT** / **SIGTERM**, and then performs a **graceful stop**.

```go
package main

import (
	"context"

	"github.com/choveylee/tgserver"
	"google.golang.org/grpc"

	pb "example.com/proto/gen" // your generated package
)

type server struct {
	pb.UnimplementedGreeterServer
}

func main() {
	ctx := context.Background()

	opt := tgserver.NewGrpcOption()
	opt.WithServiceRegistrar(func(s grpc.ServiceRegistrar) {
		pb.RegisterGreeterServer(s, &server{})
	})

	tgserver.StartGrpcServer(ctx, *opt, 50051)
}
```

You may append extra `grpc.ServerOption` values (credentials, keepalive, message limits, etc.) with `GrpcOption.WithServerOption`.

### Functional-style registration

```go
opt := tgserver.NewGrpcOption()
register := tgserver.WithGRPCRegister(func(s grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(s, &server{})
})
register(opt)
tgserver.StartGrpcServer(ctx, *opt, 50051)
```

## Features

| Area | Behavior |
|------|----------|
| **Tracing / metrics** | Registers `otelgrpc.NewServerHandler()` as a gRPC stats handler. |
| **Latency** | Unary RPC duration is recorded on a Prometheus histogram named `grpc_server_latency` (labels: `type`, `service`, `method`, `code`). Registration uses [tmetric](https://github.com/choveylee/tmetric). |
| **Access logs** | Unary interceptors emit structured logs (method, status, latency, optional JSON request/response bodies for `proto.Message` values). |
| **Recovery** | Panics are recovered and converted to `codes.Unknown` gRPC errors with log context. |
| **Internal gRPC logs** | `ReplaceGrpcLoggerV2` bridges `grpclog` to `tlog`. |

Interceptors are chained in a fixed order suitable for timing and observability (latency marker → metrics → access log → recovery → error hook).

> **Note:** Logging, metrics export, and OpenTelemetry exporters are expected to be configured in your application (for example via `tlog`, `tmetric`, and the OpenTelemetry SDK). This package wires handlers and interceptors only.

## Context helpers

The package provides utilities alongside standard `context` usage:

- **`WithValue` / `GetValue`** — attach a mutable string-keyed map to a context for intra-request data.
- **`GetValueFromMetaData`** — read incoming gRPC metadata values.
- **`SetStartTime` / `GetStartTime`** — store and read an RPC start time (used by interceptors for latency).

Prefer typed `context.WithValue` keys in application code when you do not need the string-keyed map.

## Documentation

Package documentation is available with:

```bash
go doc github.com/choveylee/tgserver
```

## Related modules

This library depends on [tlog](https://github.com/choveylee/tlog) and [tmetric](https://github.com/choveylee/tmetric) for logging and metrics primitives used by interceptors and `init`-time metric registration.
