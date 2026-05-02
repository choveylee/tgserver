# tgserver

`tgserver` is a Go library for building production-grade [gRPC](https://grpc.io/) servers with common operational capabilities, including **OpenTelemetry** instrumentation, **Prometheus** latency metrics, structured **access logging** (via [tlog](https://github.com/choveylee/tlog)), **panic recovery**, and **request-scoped context** helpers.

## Requirements

- **Go** 1.25 or later (see `go.mod`).

## Installation

```bash
go get github.com/choveylee/tgserver
```

## Quick start

After registering your gRPC services, call `StartGrpcServer`. The server listens on `tcp` on the specified port and continues serving until the context is canceled, `Serve` returns an unexpected error, or the process receives **SIGINT** or **SIGTERM**. During shutdown, it first attempts a graceful drain and then forces termination if in-flight RPCs do not complete within the configured timeout. The function returns an error if no services are registered, listener creation fails, or serving terminates unexpectedly.

```go
package main

import (
	"context"
	"log"

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

	if err := tgserver.StartGrpcServer(ctx, *opt, 50051); err != nil {
		log.Fatalf("gRPC server terminated with an error: %v", err)
	}
}
```

Additional `grpc.ServerOption` values, such as transport credentials, keepalive policies, or message size limits, may be supplied with `GrpcOption.WithServerOption`.

To route gRPC's internal process-wide logs through `tlog`, call `tgserver.ReplaceGrpcLoggerV2()` during application initialization before creating other gRPC components.

Unary request and response payload logging is disabled by default. If it is required for diagnostics, enable it explicitly with `GrpcOption.WithUnaryAccessLogPayloads(maxBytes)`. Serialized payloads are truncated before they are written to logs.

### Functional-style registration

```go
opt := tgserver.NewGrpcOption()
register := tgserver.WithGRPCRegister(func(s grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(s, &server{})
})
register(opt)
if err := tgserver.StartGrpcServer(ctx, *opt, 50051); err != nil {
	log.Fatalf("gRPC server terminated with an error: %v", err)
}
```

## Features

| Area | Behavior |
|------|----------|
| **Tracing / metrics** | Installs `otelgrpc.NewServerHandler()` as the gRPC stats handler. |
| **Latency** | Records unary and streaming RPC duration in the `grpc_server_latency` Prometheus histogram with the labels `type`, `service`, `method`, and `code`. Registration is performed through [tmetric](https://github.com/choveylee/tmetric). |
| **Access logs** | Emits structured logs for unary and streaming RPCs, including method name, status code, and latency. Unary request and response bodies are omitted by default and may be enabled explicitly with `GrpcOption.WithUnaryAccessLogPayloads`. |
| **Recovery** | Recovers from unary and streaming panics, records diagnostics on the server side, and returns a generic `codes.Unknown` gRPC error to clients. Non-status handler errors are sanitized before they are returned. |
| **Internal gRPC logs** | Applications may route `grpclog` output to `tlog` explicitly by calling `ReplaceGrpcLoggerV2` during initialization. |

Unary and streaming interceptors are chained in a fixed order designed for consistent timing and observability: latency marker → metrics → access log → recovery → error normalization.

> **Note:** Logging sinks, metrics export, and OpenTelemetry exporters are expected to be configured by the application, for example through `tlog`, `tmetric`, and the OpenTelemetry SDK. This package only wires the relevant handlers and interceptors.

## Context helpers

In addition to standard `context` usage, the package provides the following utilities:

- **`WithValue` / `GetValue`** — attach a string-keyed map to a derived context for request-scoped data without sharing mutable state across sibling contexts.
- **`GetValueFromMetaData`** — read incoming gRPC metadata values with case-insensitive key lookup.
- **`SetStartTime` / `GetStartTime`** — store and retrieve the RPC start time used by interceptors for latency measurement.

When the string-keyed helper map is unnecessary, prefer typed `context.WithValue` keys in application code.

## Documentation

Package documentation is available through:

```bash
go doc github.com/choveylee/tgserver
```

## Related modules

This library relies on [tlog](https://github.com/choveylee/tlog) and [tmetric](https://github.com/choveylee/tmetric) for the logging and metrics primitives used by the interceptors and during metric registration.
