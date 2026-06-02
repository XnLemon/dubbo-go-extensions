# Dubbo-Go Extensions

[English](./README.md) | 简体中文

社区维护的 Apache Dubbo-Go v3 扩展集合。

## 可用扩展

### 过滤器 (Filters)

- **[hystrix](./filter/hystrix/)** - 基于 hystrix-go 的熔断器过滤器

### 注册中心 (Registries)

- **[etcdv3](./registry/etcdv3/)** - Etcd v3 注册中心与服务发现

### 配置中心 (Config Centers)

## 使用方法

每个扩展都可以通过副作用导入（side-effect imports）独立使用：

```go
import (
    _ "github.com/apache/dubbo-go-extensions/filter/hystrix"
)
```

过滤器将自动注册，可在您的 dubbo-go 配置中使用。

详细文档请参见各个扩展目录。

## 许可证

Apache License 2.0 - 详见 [LICENSE](./LICENSE)
