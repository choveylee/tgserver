# tgserver

`tgserver` is a Go library for building production-ready [gRPC](https://grpc.io/) servers with common operational capabilities, including **OpenTelemetry** instrumentation, **Prometheus** latency metrics, structured **access logging** (via [tlog](https://github.com/choveylee/tlog)), **panic recovery**, and **request-scoped context** helpers.

## Requirements

- **Go** 1.25 or later (see `go.mod`).

## Installation

```bash
go get github.com/choveylee/tgserver
```

## Quick start

Register your gRPC services and then call `StartGrpcServer`. The server listens on `tcp` on the specified port and continues serving until the context is canceled, `Serve` returns an unexpected error, or the process receives **SIGINT** or **SIGTERM**. Shutdown first attempts a graceful drain and then forces termination if the drain does not complete within the configured timeout.

```go
package main

import (
	"context"

	"github.com/choveylee/tgserver"
	"google.golang.org/grpc"

	pb "example.com/proto/gen" // Generated protobuf package.
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

Additional `grpc.ServerOption` values, such as transport credentials, keepalive policies, or message size limits, may be appended with `GrpcOption.WithServerOption`.

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
| **Tracing / metrics** | Registers `otelgrpc.NewServerHandler()` as the gRPC stats handler. |
| **Latency** | Records unary and streaming RPC duration in a Prometheus histogram named `grpc_server_latency` with the labels `type`, `service`, `method`, and `code`. Registration is performed through [tmetric](https://github.com/choveylee/tmetric). |
| **Access logs** | Emits structured logs for unary and streaming RPCs, including method name, status code, latency, and optional JSON request/response bodies for unary `proto.Message` values. |
| **Recovery** | Recovers unary and streaming panics, records the panic details on the server side, and returns a generic `codes.Unknown` gRPC error to the client. |
| **Internal gRPC logs** | Redirects `grpclog` output to `tlog` through `ReplaceGrpcLoggerV2`. |

Unary and streaming interceptors are chained in a fixed order optimized for timing and observability: latency marker → metrics → access log → recovery → error normalization.

> **Note:** Logging sinks, metrics export, and OpenTelemetry exporters are expected to be configured by the application, for example through `tlog`, `tmetric`, and the OpenTelemetry SDK. This package only wires the relevant handlers and interceptors.

## Context helpers

The package also provides the following utilities alongside standard `context` usage:

- **`WithValue` / `GetValue`** — attach a string-keyed map to a derived context for request-scoped data without sharing mutable state across sibling contexts.
- **`GetValueFromMetaData`** — read incoming gRPC metadata values with case-insensitive key lookup.
- **`SetStartTime` / `GetStartTime`** — store and retrieve the RPC start time used by interceptors for latency measurement.

When the string-keyed helper map is not required, prefer typed `context.WithValue` keys in application code.

## Documentation

Package documentation is available through:

```bash
go doc github.com/choveylee/tgserver
```

## Related modules

This library depends on [tlog](https://github.com/choveylee/tlog) and [tmetric](https://github.com/choveylee/tmetric) for the logging and metrics primitives used by the interceptors and by metric registration during package initialization.
