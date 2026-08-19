# Hystrix extension

English | [简体中文](./README_CN.md)

This package provides a typed, resource-aware Hystrix extension for dubbo-go.
The extension contributes its filter automatically; users do not configure a
filter name.

## Typed options

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

The same extension option can be supplied to `server.WithExtension`. The
outer API selects consumer or provider scope; the Hystrix option does not carry
a role.

## YAML

Import the package for side-effect registration, then configure the extension
under `dubbo.extensions`:

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

The service or method key is preserved as one exact key, including dots and
colons. A service-level key such as `greet.GreetService` can be used when the
same settings apply to all methods.

Defaults are applied first, YAML second, and typed options last. Runtime state
is isolated by Client/Server and by Resource.
