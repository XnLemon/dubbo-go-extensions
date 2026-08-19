# Hystrix 扩展

[English](./README.md) | 简体中文

该包为 dubbo-go 提供 typed、resource-aware 的 Hystrix 扩展。扩展会自动贡献
Filter，用户不再手写 filter name。

## Typed option

```go
import (
    "dubbo.apache.org/dubbo-go/v3/client"
    "github.com/apache/dubbo-go-extensions/filter/hystrix"
)

cli, err := client.NewClient(
    client.WithExtension(
        hystrix.WithConfig(
            hystrix.WithCommandName("greet.GreetService:::Greet"),
            hystrix.WithTimeout(1000),
            hystrix.WithMaxConcurrentRequests(10),
            hystrix.WithRequestVolumeThreshold(5),
            hystrix.WithSleepWindow(5000),
            hystrix.WithErrorPercentThreshold(50),
        ),
    ),
)
```

同一个 `hystrix.WithConfig(...)` 也可以传给 `server.WithExtension`。consumer 或
provider 角色由外层 API 决定，Hystrix option 本身不携带 role。

## YAML

只需 side-effect import 扩展包，然后在 `dubbo.extensions` 下配置：

```yaml
dubbo:
  extensions:
    hystrix:
      consumer:
        "greet.GreetService:::Greet":
          timeout: 1000
          max-concurrent-requests: 10
          request-volume-threshold: 5
          sleep-window: 5000
          error-percent-threshold: 50
      provider:
        "greet.GreetService:::Greet":
          timeout: 1500
```

服务或方法 key 会作为完整 key 保留，`.` 和 `:` 不会被拆分。只配置
`greet.GreetService` 可以为该服务的所有方法提供默认配置。

配置优先级为：默认值 < YAML < typed option。每个 Client/Server 以及每个 Resource
拥有隔离的运行时状态。
