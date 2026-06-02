# Etcdv3 注册中心

[English](./README.md) | 简体中文

基于 etcd v3 的 [dubbo-go](https://github.com/apache/dubbo-go) v3 注册中心与服务发现扩展。

## 安装

```bash
go get github.com/apache/dubbo-go-extensions/registry/etcdv3
```

## 快速开始

一个基于 etcd 注册中心的完整 RPC 调用示例。

### 前置条件

- Go 1.25+
- etcd 运行在 `127.0.0.1:2379`

### 1. 导入

```go
import (
    _ "github.com/apache/dubbo-go-extensions/registry/etcdv3"
)
```

`init()` 函数会自动注册 **etcdv3 注册中心**和 **etcdv3 服务发现**。

### 2. Provider（服务提供者）

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

// 必要的 dubbo-go 扩展导入
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
    // 重要：使用 WithProtocol 显式限制协议。
    // 不指定时 triple 协议可能被间接导入自动注册，
    // 导致 "marshal message: []interface{} doesn't implement proto.Message" 错误。
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

### 3. Consumer（服务消费者）

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

    // 重要：显式使用 dubbo 协议。不指定时客户端默认使用 triple 协议，
    // 需要 protobuf 消息格式，与 hessian2 不兼容。
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

    fmt.Println(resp.Greeting) // 输出: Hello, World!
}
```

## YAML 配置

### `dubbogo.yaml`

```yaml
dubbo:
  registries:
    etcd:
      protocol: etcdv3
      address: 127.0.0.1:2379
      timeout: 5s
```

多 etcd 节点：

```yaml
dubbo:
  registries:
    etcd:
      protocol: etcdv3
      address: 127.0.0.1:2379,127.0.0.1:2380,127.0.0.1:2381
      timeout: 5s
```

## 服务发现 API

服务发现自动以 `"etcdv3"` 为键注册：

```go
import (
    _ "github.com/apache/dubbo-go-extensions/registry/etcdv3"

    "dubbo.apache.org/dubbo-go/v3/common"
    "dubbo.apache.org/dubbo-go/v3/common/constant"
    "dubbo.apache.org/dubbo-go/v3/common/extension"
    "dubbo.apache.org/dubbo-go/v3/registry"
)

// 构建指向 etcd 的 URL
sdURL, _ := common.NewURL(
    "etcdv3://127.0.0.1:2379",
    common.WithParamsValue(constant.RegistryKey, constant.EtcdV3Key),
)

// 获取服务发现实例
sd, _ := extension.GetServiceDiscovery(sdURL)

// 注册服务实例
sd.Register(&registry.DefaultServiceInstance{
    ID:          "instance-1",
    ServiceName: "my-service",
    Host:        "10.0.0.1",
    Port:        8080,
    Enable:      true,
    Healthy:     true,
    Metadata:    map[string]string{"version": "1.0.0"},
})

// 获取某服务的所有实例
instances := sd.GetInstances("my-service")
```

## 重要提示

### 协议限制

Provider 端务必使用 `dubbo.WithProtocol(protocol.WithDubbo(), ...)` 显式限制协议。dubbo-go 的 `protocol/options.go` 会间接导入 `protocol/triple` 包，导致 triple 协议被自动注册。不显式限制时，Provider 会同时暴露 dubbo 和 triple 两种协议，Consumer 可能报错：

```
marshal message: []interface {} doesn't implement proto.Message
```

### 客户端协议

Consumer 端需使用 `client.WithClientProtocolDubbo()`。不指定时客户端默认走 triple 协议，当 Provider 只导出 dubbo 时会因协议不匹配而找不到服务提供者。

## 注册名称

- **注册中心**: `"etcdv3"` — 使用 `registry.WithEtcdV3()` 或 `registry.WithRegistry("etcdv3")`
- **服务发现**: `"etcdv3"` — 使用 `constant.EtcdV3Key`
