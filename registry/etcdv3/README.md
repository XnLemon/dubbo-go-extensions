# Etcdv3 Registry

English | [简体中文](./README_CN.md)

Etcd v3 registry and service discovery extension for [dubbo-go](https://github.com/apache/dubbo-go) v3.

## Installation

```bash
go get github.com/apache/dubbo-go-extensions/registry/etcdv3
```

## Quick Start

A complete RPC example using etcd as the registry center.

### Prerequisites

- Go 1.25+
- etcd running on `127.0.0.1:2379`

### 1. Import

```go
import (
    _ "github.com/apache/dubbo-go-extensions/registry/etcdv3"
)
```

The `init()` functions automatically register both **etcdv3 registry** and **etcdv3 service discovery**.

### 2. Provider

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"
)

import (
    _ "github.com/apache/dubbo-go-extensions/registry/etcdv3"

    "dubbo.apache.org/dubbo-go/v3"
    "dubbo.apache.org/dubbo-go/v3/protocol"
    "dubbo.apache.org/dubbo-go/v3/registry"
)

// Blank imports for required dubbo-go extensions
import (
    _ "dubbo.apache.org/dubbo-go/v3/filter/echo"
    _ "dubbo.apache.org/dubbo-go/v3/filter/graceful_shutdown"
    _ "dubbo.apache.org/dubbo-go/v3/metadata/mapping/metadata"
    _ "dubbo.apache.org/dubbo-go/v3/metadata/report/etcd"
    _ "dubbo.apache.org/dubbo-go/v3/protocol/dubbo"
    _ "dubbo.apache.org/dubbo-go/v3/proxy/proxy_factory"
    _ "dubbo.apache.org/dubbo-go/v3/registry/directory"
    _ "dubbo.apache.org/dubbo-go/v3/registry/protocol"
    _ "dubbo.apache.org/dubbo-go/v3/registry/servicediscovery"
)

type GreetServiceImpl struct{}

func (s *GreetServiceImpl) Reference() string {
    return "greet.GreetService"
}

func (s *GreetServiceImpl) Greet(ctx context.Context, req *GreetRequest) (*GreetResponse, error) {
    return &GreetResponse{Greeting: "Hello, " + req.Name + "!"}, nil
}

func main() {
    // IMPORTANT: Use WithProtocol to explicitly restrict protocols.
    // Without this, the triple protocol may be auto-registered via transitive imports,
    // causing "marshal message: []interface{} doesn't implement proto.Message" errors.
    ins, _ := dubbo.NewInstance(
        dubbo.WithName("greet-provider"),
        dubbo.WithProtocol(
            protocol.WithDubbo(),
            protocol.WithPort(20000),
        ),
        dubbo.WithRegistry(
            registry.WithEtcdV3(),
            registry.WithAddress("127.0.0.1:2379"),
            registry.WithTimeout(5*time.Second),
        ),
    )

    srv, _ := ins.NewServer()
    srv.RegisterService(&GreetServiceImpl{})

    go srv.Serve()

    ch := make(chan os.Signal, 1)
    signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
    <-ch
}
```

### 3. Consumer

```go
package main

import (
    "context"
    "fmt"
    "time"
)

import (
    _ "github.com/apache/dubbo-go-extensions/registry/etcdv3"

    "dubbo.apache.org/dubbo-go/v3"
    "dubbo.apache.org/dubbo-go/v3/client"
    "dubbo.apache.org/dubbo-go/v3/registry"
)

import (
    _ "dubbo.apache.org/dubbo-go/v3/cluster/cluster/failover"
    _ "dubbo.apache.org/dubbo-go/v3/cluster/loadbalance/random"
    _ "dubbo.apache.org/dubbo-go/v3/filter/echo"
    _ "dubbo.apache.org/dubbo-go/v3/filter/graceful_shutdown"
    _ "dubbo.apache.org/dubbo-go/v3/metadata/mapping/metadata"
    _ "dubbo.apache.org/dubbo-go/v3/metadata/report/etcd"
    _ "dubbo.apache.org/dubbo-go/v3/protocol/dubbo"
    _ "dubbo.apache.org/dubbo-go/v3/proxy/proxy_factory"
    _ "dubbo.apache.org/dubbo-go/v3/registry/directory"
    _ "dubbo.apache.org/dubbo-go/v3/registry/protocol"
    _ "dubbo.apache.org/dubbo-go/v3/registry/servicediscovery"
)

func main() {
    ins, _ := dubbo.NewInstance(
        dubbo.WithName("greet-consumer"),
        dubbo.WithRegistry(
            registry.WithEtcdV3(),
            registry.WithAddress("127.0.0.1:2379"),
        ),
    )

    // IMPORTANT: Explicitly use dubbo protocol. Without this, the client
    // defaults to triple which requires protobuf messages.
    cli, _ := ins.NewClient(
        client.WithClientProtocolDubbo(),
    )

    conn, _ := cli.Dial("greet.GreetService",
        client.WithRegistry(
            registry.WithEtcdV3(),
            registry.WithAddress("127.0.0.1:2379"),
        ),
        client.WithSerialization("hessian2"),
        client.WithIDL("non-IDL"),
    )

    req := &GreetRequest{Name: "World"}
    resp := &GreetResponse{}
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    conn.CallUnary(ctx, []any{req}, resp, "Greet")

    fmt.Println(resp.Greeting) // Output: Hello, World!
}
```

## Configuration via YAML

### `dubbogo.yaml`

```yaml
dubbo:
  registries:
    etcd:
      protocol: etcdv3
      address: 127.0.0.1:2379
      timeout: 5s
```

Multiple etcd endpoints:

```yaml
dubbo:
  registries:
    etcd:
      protocol: etcdv3
      address: 127.0.0.1:2379,127.0.0.1:2380,127.0.0.1:2381
      timeout: 5s
```

## Service Discovery API

Service discovery is automatically registered under the key `"etcdv3"`:

```go
import (
    _ "github.com/apache/dubbo-go-extensions/registry/etcdv3"

    "dubbo.apache.org/dubbo-go/v3/common"
    "dubbo.apache.org/dubbo-go/v3/common/constant"
    "dubbo.apache.org/dubbo-go/v3/common/extension"
    "dubbo.apache.org/dubbo-go/v3/registry"
)

// Build a URL pointing to etcd
sdURL, _ := common.NewURL(
    "etcdv3://127.0.0.1:2379",
    common.WithParamsValue(constant.RegistryKey, constant.EtcdV3Key),
)

// Get the service discovery instance
sd, _ := extension.GetServiceDiscovery(sdURL)

// Register a service instance
sd.Register(&registry.DefaultServiceInstance{
    ID:          "instance-1",
    ServiceName: "my-service",
    Host:        "10.0.0.1",
    Port:        8080,
    Enable:      true,
    Healthy:     true,
    Metadata:    map[string]string{"version": "1.0.0"},
})

// Get all instances for a service
instances := sd.GetInstances("my-service")
```

## Important Notes

### Protocol Restriction

Always use `dubbo.WithProtocol(protocol.WithDubbo(), ...)` on the provider side. The `protocol/triple` package is imported transitively by dubbo-go and will auto-register the triple protocol. Without explicit restriction, the provider will export on both dubbo and triple, and the consumer may fail with:

```
marshal message: []interface {} doesn't implement proto.Message
```

### Client Protocol

Use `client.WithClientProtocolDubbo()` when creating the client. Without it, the client defaults to triple protocol and will fail to find matching providers when only dubbo is exported.

## Registry Keys

- **Registry**: `"etcdv3"` — use `registry.WithEtcdV3()` or `registry.WithRegistry("etcdv3")`
- **Service Discovery**: `"etcdv3"` — use `constant.EtcdV3Key`
