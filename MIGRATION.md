# Migration guide

## v0.14.0

### `Dial()` is replaced by `DialContext()`

Update:
```diff
-cli, err := mqtt.Dial(url)
+cli, err := mqtt.DialContext(ctx, url)
 if err != nil { ... }
 _, err := cli.Connect(ctx, ...)
```

If `mqtt.DialerFunc` is used, update:
```diff
-mqtt.DialerFunc(func() (*mqtt.BaseClient, error) {
+mqtt.DialerFunc(func(ctx context.Context) (*mqtt.BaseClient, error) {
```

## v0.12.0

### `Dial()` returns `*BaseClient` instead of `ClientCloser`

Update variable type if needed.

### Second argument of `RetryClient.SetClient()` requires `*BaseClient` instead of `ClientCloser`

Store client passed to `SetClient` as `*BaseClient` or convert it by the type assertion like `cli.(*BaseClient)`.

### `Client.Subscribe()` returns resultant subscription information as a first return value

Update:
```diff
-err := cli.Subscribe(...)
+_, err := cli.Subscribe(...)
```
