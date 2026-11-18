# tgserver

`tgserver` is a Go library for building production-grade [gRPC](https://grpc.io/) servers with common operational capabilities, including **OpenTelemetry** instrumentation, **Prometheus** latency metrics, structured **access logging** (via [tlog](https://github.com/choveylee/tlog)), **panic recovery**, and **request-scoped context** helpers.

## Requirements

- **Go** 1.27.4 or later.

## Installation

```bash
go get github.com/choveylee/tgserver
```

## Quick start

Call `StartGrpcServer` with service registration callbacks and a TCP port. It serves until the context is canceled, serving fails, or the process receives SIGINT or SIGTERM. Cancellation during registration stops startup after the active callback returns. Shutdown waits up to 30 seconds for in-flight RPCs by default. Change the limit with `WithGracefulShutdownTimeout`. When the limit expires, the listener and the TCP connections it still owns close, including pending handshakes. The returned error matches `context.DeadlineExceeded` through `errors.Is`. Running handlers should respond to context cancellation to complete cleanup.

Set transport credentials with `WithTransportCredentials`. Custom credentials receive a `net.Conn` and must use `WithTransportCredentials` if they return `credentials.ErrConnDispatched`. This lets the listener release ownership and keep the dispatched socket open during forced shutdown. Standard TLS credentials also work with `WithServerOption(grpc.Creds(...))`.

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

	options := tgserver.NewGrpcOption()
	options.WithServiceRegistrar(func(registrar grpc.ServiceRegistrar) {
		pb.RegisterGreeterServer(registrar, &server{})
	})

	if err := tgserver.StartGrpcServer(ctx, *options, 50051); err != nil {
		log.Fatalf("gRPC server terminated with an error: %v", err)
	}
}
```

Use `WithServerOption` for keepalive and message size limits. Register custom interceptors with `grpc.ChainUnaryInterceptor` and `grpc.ChainStreamInterceptor` so the server's panic recovery and timing surround them. Standalone interceptor options return a startup error. Registration callbacks receive a `grpc.ServiceRegistrar`, which also supports gRPC reflection through `GetServiceInfo`. Service registration checks the implementation and takes a snapshot of the method and stream descriptors.

To route gRPC's internal process-wide logs through `tlog`, call `tgserver.ReplaceGrpcLoggerV2()` during application initialization before creating other gRPC components.

Use `WithUnaryAccessLogPayloads(maxBytes)` to enable unary protobuf diagnostics. Request diagnostics capture the request before the handler runs. Redaction removes common sensitive field and map-key names, opaque byte fields, and the contents of `Any` envelopes. Each logged payload has a byte limit. For sensitive values stored under other names, keep the default metadata-only logs.

### Functional-style registration

```go
options := tgserver.NewGrpcOption()
register := tgserver.WithGRPCRegister(func(registrar grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(registrar, &server{})
})
register(options)
if err := tgserver.StartGrpcServer(ctx, *options, 50051); err != nil {
	log.Fatalf("gRPC server terminated with an error: %v", err)
}
```

## Features

| Area | Behavior |
|------|----------|
| **Tracing / metrics** | Installs the OpenTelemetry server stats handler with normalized method names and status codes. |
| **Latency** | Records unary and streaming RPC duration in the `grpc_server_latency` Prometheus histogram with the labels `type`, `service`, `method`, and `code`. Registration is performed through [tmetric](https://github.com/choveylee/tmetric). |
| **Access logs** | Emits structured logs for unary and streaming RPCs, including method name, status code, and latency. Unary request and response bodies are omitted by default and may be enabled explicitly with `GrpcOption.WithUnaryAccessLogPayloads`. |
| **Recovery** | Recovers from handler and chained-interceptor panics, logs the panic type and call site, and returns a generic `codes.Unknown` status. Explicit gRPC statuses retain their public message and details. |
| **Internal gRPC logs** | Applications may route `grpclog` output to `tlog` explicitly by calling `ReplaceGrpcLoggerV2` during initialization. |

The access log and Prometheus histogram use the final gRPC completion status and duration, including response encoding and transport errors. By default, gRPC returns `Unimplemented` for unregistered methods before running interceptors. The server records these rejections under the `unknown` labels when gRPC emits trailers. A custom `grpc.UnknownServiceHandler` runs through the stream interceptor chain and uses the same bounded labels.

> **Note:** Logging sinks, metrics export, and OpenTelemetry exporters are expected to be configured by the application, for example through `tlog`, `tmetric`, and the OpenTelemetry SDK. This package only wires the relevant handlers and interceptors.

## Context helpers

In addition to standard `context` usage, the package provides the following utilities:

- **`WithValue` / `GetValue`** — attach a string-keyed value to a derived context. Children can override a key independently.
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
