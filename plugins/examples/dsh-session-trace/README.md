# DeepSeek Harness Session Trace

The plugin uses the `x-deepseek-harness-session-id` request header as a trace ID candidate. Requests in the same AxonHub project with the same DSH session ID share a trace. Explicit trace headers and AxonHub's built-in client detection take precedence.

From the AxonHub repository root, package the example with:

```sh
go run -C cmd/pack-plugin . ../../plugins/examples/dsh-session-trace
```

Upload `plugins/examples/dsh-session-trace.axplugin` under **Plugins** and enable it. The plugin takes effect on new gateway requests after the plugin manager refreshes.
