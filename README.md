# Dubbo-Go Extensions

English | [简体中文](./README_CN.md)

Community-maintained extensions for Apache Dubbo-Go v3.

## Available Extensions

### Filters

- **[hystrix](./filter/hystrix/)** - Circuit breaker filter using hystrix-go

### Registries

- **[etcdv3](./registry/etcdv3/)** - Etcd v3 registry and service discovery

### Config Centers

## Usage

Each extension can be used independently via side-effect imports:

```go
import (
    _ "github.com/apache/dubbo-go-extensions/filter/hystrix"
)
```

The filter will be automatically registered and available for use in your dubbo-go configuration.

See individual extension directories for detailed documentation.

## License

Apache License 2.0 - See [LICENSE](./LICENSE) for details.
