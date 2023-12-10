#### go mod tidy err
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc tested by   
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc.test imports   
google.golang.org/grpc/interop imports   
golang.org/x/oauth2/google imports   
cloud.google.com/go/compute/metadata: ambiguous import: found package cloud.google.com/go/compute/metadata in multiple modules:   
cloud.google.com/go v0.26.0 (/Users/phoenix/Documents/rcrai/pkg/mod/cloud.google.com/go@v0.26.0/compute/metadata)   
cloud.google.com/go/compute/metadata v0.2.3 (/Users/phoenix/Documents/rcrai/pkg/mod/cloud.google.com/go/compute/metadata@v0.2.3)   

#### fix: go get cloud.google.com/go/compute/metadata; go mod tidy;