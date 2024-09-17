# Config

The HyperSDK defines its own configuration and enables Services to register their own config as well.

See the HyperSDK's config [here](../../vm/config.go).

## Service Configs

Every service includes a `namespace` and `defaultConfig` value registered via:

```golang
func NewOption[T any](namespace string, defaultConfig T, optionFunc OptionFunc[T]) Option { ... }
```

Service configs are nested into the HyperSDK's config and option namespaces are required to be unique, when they're registered.

As an example, the `externalsubscriber` service can be configured with the following JSON:

```json
{
  "services": {
    "externalSubscriber": {
      "enabled": true,
      "serverAddress": "127.0.0.1:57201"
    }
  }
}
```

The HyperSDK will unmarshal any JSON provided under the service's namespace directly into the default config value, so that any field that's not explicitly populated falls back to the default value. A further improvement would be to utilize a tool like [Viper](https://github.com/spf13/viper), so that there's a richer feature set including checking whether a flag was set or is just falling back to the default value.

For a programmatic example of passing the chain config in via `tmpnet`, see the integration tests [here](../../tests/integration/integration.go).
