# Realtime Gateway

Realtime uses an independent bidirectional WebSocket session, separate from Chat, Responses WebSocket, TTS and STT. The development branch is `feat/qwen-realtime`.

## Verification and release gate

Use the actual evidence recorded in `.agents/summary/realtime-delivery.md`. Mock tests establish local behavior, not official account availability, audio quality or provider billing accuracy. Every supported model needs its own live acceptance result.

The environment's OpenAI credential belongs to a third-party relay and must not be sent to OpenAI's official host. No Bailian key or Workspace is available. Official live acceptance for both OpenAI models and all Qwen models remains unexecuted and blocks production approval. The deployed reverse proxy, Redis failover behavior, audio intelligibility/duration and actual provider invoices also require acceptance.

## Configure through the admin UI

1. Use the existing `openai` or `bailian` channel and its credential manager.
2. In the channel's main form, select `openai/realtime-ws` for OpenAI or `qwen/realtime-ws` for Bailian. The channel stores this independently in `settings.realtime`: `enabled`, `protocol`, `baseURL`, `path`, and optional `handshakeTimeoutMs`. Set a 1000–60000 ms handshake timeout or leave it empty to inherit the server setting. Keep the channel's HTTP Base URL intact; the Realtime WS address has its own field.
3. OpenAI uses `wss://api.openai.com` with `/v1/realtime`. The initial model set is `gpt-realtime-2.1` and `gpt-realtime-2.1-mini`; deprecated models are not new defaults.
4. For Bailian, copy the trusted API Host for the selected business workspace from the provider console and use its WSS address with `/api-ws/v1/realtime`. Hosts are limited to `cn-beijing` and `ap-southeast-1` under `maas.aliyuncs.com`; preserve the console's host label, including an `llm-` prefix when present. Workspace, key and region must match. Supported models are `qwen3.8-omni-flash-realtime`, `qwen3.5-omni-plus-realtime` and `qwen3.5-omni-flash-realtime`.
5. Configure the channel's supported-model whitelist, mappings, model associations and API Key Profile permissions. Realtime protocol selection belongs to the channel form, not endpoint management.
6. Configure the independent realtime price items using the current official rate and the installation's currency. Do not substitute historical text-model price codes or mix USD and CNY.
7. Channel testing verifies an actual WebSocket handshake, configuration acknowledgement and a lightweight conversation-item acknowledgement, without starting model generation. It does not certify audio or invoices.

Endpoint management does not offer Realtime protocols. Sending `openai/realtime-ws` or `qwen/realtime-ws` through `saveChannelEndpoints` or another endpoint-writing input is rejected with HTTP `422` and code `ENDPOINT_REALTIME_NOT_ALLOWED`; the server does not ignore or persist it. Set `settings.realtime.enabled` to `false`, or switch the main form to a non-Realtime protocol, to stop new Realtime sessions without deleting HTTP endpoints.

Selecting Realtime makes the channel exclusive to realtime requests. Its HTTP base URL, custom endpoints, and model settings remain stored. Endpoint management indicates that the ordinary endpoints are inactive. Selecting an ordinary protocol restores its default and custom endpoints. Use separate channels when serving both ordinary and realtime traffic.

Realtime channel configuration is available on the existing OpenAI and Bailian channel types. xAI Subscription channels use their provider-managed endpoint and reject custom Realtime endpoint input.

The existing startup data-migration pipeline moves legacy Realtime entries from `endpoints` to `settings.realtime` before the channel cache loads. Each channel update moves the configuration and removes the legacy entries atomically, while preserving ordinary endpoints and unrelated settings. An existing channel-owned Realtime setting wins, including an explicit disabled setting. Re-running the migration makes no further channel writes. This changes the JSON configuration shape and adds no database table or column.

Backup restore uses the same conversion for valid legacy endpoints. At startup, an invalid legacy configuration is left in place and logged, and its legacy endpoint stays inactive. A restore containing invalid Realtime configuration returns an error and rolls back.

This follows the existing database auto-migration setting: if it is disabled, run a controlled startup with migration enabled before serving migrated Realtime channels. Drain Realtime traffic during a mixed-version deployment and finish upgrading all instances before reopening it; older binaries cannot read the new channel configuration.

## Client contract

Connect to `GET /v1/realtime?model={gateway-model-or-alias}` using WebSocket Upgrade and `Authorization: Bearer {AxonHub API Key}`. The first release supports server clients only. Browser Origin headers, URL credentials and client-selected upstream hosts are rejected. Authentication never falls back to No-Auth.

The native `/qwen/v1/realtime?model={model}` entry uses Qwen event names and the same authentication, routing, quota, price and connection checks. It can only select Bailian channels with enabled Realtime configuration and still validates the implemented safe capability subset.

Choose the entry by the event protocol the client speaks. Standard OpenAI Realtime GA clients use `/v1/realtime`; if the selected model routes to Qwen, the gateway converts the supported events to GA. Use `/qwen/v1/realtime` only when the client needs native Qwen event names. That entry routes only to enabled Bailian Realtime channels and does not perform GA event conversion.

The SDK Base URLs below point to the public AxonHub address, not the upstream Realtime Host configured on a channel. Choose the Base URL that matches the path suffix appended by your SDK, and make sure it preserves the Base URL path prefix.

| Client protocol | WebSocket entry | SDK Base URL example | Use when |
| --- | --- | --- | --- |
| OpenAI Realtime GA | `wss://gateway.example/v1/realtime?model={model}` | If the SDK appends `/realtime`, use `https://gateway.example/v1`; if it appends `/v1/realtime`, use `https://gateway.example` | The client speaks OpenAI GA events; routing can reach OpenAI or convert a Qwen model to GA events |
| Native Qwen Realtime | `wss://gateway.example/qwen/v1/realtime?model={model}` | If the SDK appends `/realtime`, use `https://gateway.example/qwen/v1`; if it appends `/v1/realtime`, use `https://gateway.example/qwen` | The client speaks native Qwen events and should route only to a Bailian Qwen upstream |

Both entries use the AxonHub API key in an `Authorization: Bearer ...` header. If the client cannot preserve a Base URL path prefix, use a client that accepts a complete WebSocket URL and connect directly to the entry above.

For OpenAI, valid GA events, extension fields and event order are forwarded without unnecessary JSON rewriting or resampling. No `OpenAI-Beta: realtime=v1` header is sent. When `session.created` contains the complete effective GA audio configuration, its confirmed defaults can be used directly. Configuration changes require `session.update` and its `session.updated` acknowledgement before using the new parameters. Session model changes, credential overrides and caller-supplied safety identity are prohibited. The upstream Safety Identifier is a one-way digest derived from the authenticated Gateway key and user/project identity.

Qwen Realtime models support `server_vad`. Qwen3.8 Omni Flash, Qwen3.5 Omni Plus and Qwen3.5 Omni Flash also support `semantic_vad`. Semantic VAD thresholds range from -1 to 1 and silence duration ranges from 200 to 6000 milliseconds. Other models accept only VAD types declared in their capability table.

Qwen has its own adapter. GA text events use `response.output_text.delta/done`, audio uses `response.output_audio.delta/done`, and audio transcripts use `response.output_audio_transcript.delta/done`. Function call IDs, argument deltas, final arguments and results remain associated. Unsupported fields are rejected explicitly.

Remote MCP, video, multichannel audio, browser ephemeral tokens and Preview compatibility are not default first-release capabilities. Upstream acceptance of a field is not evidence of Gateway support. Native OpenAI image usage requires complete image/cache prices and usage breakdowns.

Minimal Node.js server example (`npm install ws`):

```js
import WebSocket from 'ws';
const ws = new WebSocket('wss://axonhub.example/v1/realtime?model=voice', {
  headers: { Authorization: `Bearer ${process.env.AXONHUB_API_KEY}` },
});
ws.on('message', (bytes) => {
  const e = JSON.parse(bytes.toString());
  if (e.type === 'session.created') ws.send(JSON.stringify({
    type: 'session.update', session: {
      type: 'realtime', output_modalities: ['text'],
      audio: { input: { turn_detection: null } },
    },
  }));
  if (e.type === 'session.updated') {
    ws.send(JSON.stringify({ type: 'conversation.item.create', item: {
      type: 'message', role: 'user',
      content: [{ type: 'input_text', text: 'Hello. Please introduce yourself briefly.' }],
    }}));
    ws.send(JSON.stringify({ type: 'response.create' }));
  }
  if (e.type === 'response.output_text.delta') process.stdout.write(e.delta);
  if (e.type === 'response.done') ws.close();
});
```

For native Qwen, use the Qwen route and `session.modalities: ['text']`, `session.turn_detection: null`, without GA `session.type`. Native text events remain `response.text.delta/done`.

## Usage, pricing and quotas

Audio input or `response.create` first reserves a metadata-only audit record, which is associated with `response.created`. Confirmed `response.done` usage is billed once, and multiple responses create independent records. Each session allows at most 512 audit reservations, including completed, cleared and pending records; deduplication is bounded too. Confirmed usage survives disconnection. An unfinished response is failed with `usage_confirmed: false`; no fabricated zero-usage ledger record is created.

Realtime pricing is independent of historical prompt/completion codes. It distinguishes input text/audio, output text/audio and OpenAI image input plus cached text/audio/image input. Cached tokens are subsets of the corresponding input modality and are subtracted from uncached quantities. Decimal CostItem subtotals and the frozen price reference support auditing. The existing numeric `totalCost` representation remains for storage compatibility.

Realtime voice modalities support partial pricing. In the base price or active schedule override, each configured positive voice rate bills only its matching CostItem; unpriced voice modalities remain in UsageLog totals but are excluded from cost. If an effective OpenAI Realtime price has any Realtime rates, it must still include all four input-image and cached input text/audio/image rates; voice/audio rates may be partial. With no Realtime-specific rates, sessions remain available, usage is recorded and cost remains unset. Realtime Translation retains its separate per-minute audio-sample rate.

Missing voice rates do not block route selection or upstream WebSocket setup. OpenAI image/cache rates that fail the completeness requirement still reject the route before dialing. Partial voice pricing means the bill covers only configured voice modalities. Compare UsageLog's total usage with its CostItem breakdown so unpriced modalities are not mistaken for provider-reported zero usage.

Qwen3.8 bills speech and its transcript separately when the corresponding output rates are configured. Qwen3.5's model billing policy preserves actual speech-transcript tokens while setting their billable text quantity to zero; text-only and function-argument output still use the text rate when configured. With no Realtime rates, cost remains unset; with partial rates, only priced modalities are charged. Session prices are frozen so reloading or disabling a channel cannot lose confirmed usage.

Quota checks run before connection establishment and periodically during the session, including autonomous VAD generation, and prevent subsequent input. This is usage enforcement rather than a provider-side prepaid spending lock: an in-progress response can exceed the remaining budget before final usage is known. Confirm this boundary against the deployment's budget requirements before rollout.

## Deployment and limits

```yaml
server:
  realtime:
    handshake_timeout: 15s
    idle_timeout: 2m
    max_duration: 15m
    limits:
      global: 32
      project: 8
      api_key: 1
      channel: 8
      connections_per_minute: 20
```

Use the existing YAML/environment configuration loader, for example `AXONHUB_SERVER_REALTIME_MAX_DURATION=15m` with the prefix used by your deployment. No separate hidden feature flag is required; the channel's explicit `settings.realtime.enabled` setting enables the capability.

The existing `cache.redis` configuration enables shared admission, connected lazily on the first realtime request. One same-slot Lua script atomically acquires global/project/key/channel concurrency and per-key connection-rate admission. Leases outlive the session hard deadline and expire after a crash. Unavailable Redis rejects new sessions. Without Redis, limits apply to one process only; multiple instances must share Redis and identical limits. Redis persistence and failover write-loss risk are deployment acceptance concerns.

A reconnect encountering a concurrency limit may wait up to one second in the same HTTP handshake for the previous transport to finish. It never evicts an active session or increases the configured limits. Each process admits at most one waiter per key and at most the configured global concurrency count of waiters; rate denials and Redis failures are not retried. Connection leases are released after both sockets close and transport workers exit, before potentially slower audit finalization. Audit finalization remains part of the bounded shutdown drain.

Backpressure keeps its one-second queue wait and initiates a `1013` Close with reason `backpressure`. The gateway stops forwarding media, serializes the Close behind an in-flight write, and allows up to one additional second for writing and peer acknowledgement before closing TCP. Normal and internal-error closes use `1000` and `1011` respectively. A peer that remains unable to read can still observe an abnormal transport close; the gateway logs an explicit close-flush timeout and releases resources rather than waiting indefinitely. Authentication errors remain HTTP responses before upgrade.

Messages are limited to 256 KiB, decoded audio chunks to 64 KiB, and each directional send queue to eight messages. Queue exhaustion closes the session instead of dropping media. Handshake, acknowledgement, idle and total-duration timeouts are bounded. Server shutdown cancels both ends instead of waiting indefinitely for long-lived sessions.

Nginx example, added to an existing TLS configuration:

```nginx
# http {} context
map $http_upgrade $realtime_connection {
    default upgrade;
    '' close;
}
# existing server {} context
location /v1/realtime {
    proxy_pass http://axonhub:8090;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $realtime_connection;
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Host $host;
    proxy_read_timeout 20m;
    proxy_send_timeout 20m;
    proxy_buffering off;
}
```

Containers need no extra port. Load balancers must support Upgrade. HTTP health checks establish process health, not Realtime endpoint correctness.

The existing OpenTelemetry Provider exports active sessions, connection/admission/handshake events, session duration, first-output latency and modality token counters. Request/usage pages retain confirmed charges. Logs include channel/key-record IDs, duration and closure state; media, Base64, conversation text and long-lived keys are not recorded.

## Live smoke and rollback

Set `AXONHUB_API_KEY` privately, and run from the repository root:

```sh
go run ./scripts/realtime-smoke -model gpt-realtime-2.1-mini -audio-pcm /secure/input-24k-mono.pcm
go run ./scripts/realtime-smoke -model gpt-realtime-2.1 -audio-pcm /secure/input-24k-mono.pcm
go run ./scripts/realtime-smoke -model qwen3.8-omni-flash-realtime -audio-pcm /secure/input-24k-mono.pcm
go run ./scripts/realtime-smoke -qwen-native -model qwen3.8-omni-flash-realtime -audio-pcm /secure/input-24k-mono.pcm
```

The recording must be headerless mono PCM16 LE at 24 kHz. The script tests configuration, text, audio input/output, continuous turns, function result submission and continuation, printing confirmed usage only. It does not play or save generated audio. Intelligibility, duration, VAD, cancellation and hostile networks require separate acceptance.

Back up the database and channel configuration. Enable the channel's Realtime configuration for one restricted project with low concurrency, short duration and small quotas, then compare provider invoices before expanding. To roll back, disable `settings.realtime.enabled` or the relevant channel, wait for bounded sessions or stop the server to cancel them, then revert the application. There is no table or column migration, but the JSON configuration shape changes: an older application that reads only Realtime endpoints will not use `settings.realtime`. If that older version must continue serving Realtime, restore its matching channel-configuration backup after switching versions; do not write Realtime back through the current endpoint API. Check price-enum compatibility before running older code, and retain confirmed invoice audit records.

## Official sources

- [OpenAI WebSockets](https://developers.openai.com/api/docs/guides/voice-websockets)
- [OpenAI GA client events](https://developers.openai.com/api/reference/resources/realtime/client-events)
- [OpenAI GA server events](https://developers.openai.com/api/reference/resources/realtime/server-events)
- [GPT-Realtime-2.1](https://developers.openai.com/api/docs/models/gpt-realtime-2.1)
- [GPT-Realtime-2.1 mini](https://developers.openai.com/api/docs/models/gpt-realtime-2.1-mini)
- [Bailian WebSockets](https://help.aliyun.com/zh/model-studio/omni-realtime-websocket-access)
- [Qwen client events](https://help.aliyun.com/zh/model-studio/client-events)
- [Qwen server events](https://help.aliyun.com/zh/model-studio/server-events)
- [Qwen3.8](https://help.aliyun.com/zh/model-studio/qwen3-8-omni-flash-realtime)

Run `go run ./scripts/realtime-proxy-check` for a reproducible Nginx check (Docker and `nginx:alpine` required). It uses a local Mock echo service to verify real HTTP 101, Authorization forwarding, query preservation and bidirectional frames, without contacting a provider.

## Accounting policy and explicit limits

Unknown usage is never estimated by modality ratios or billed as ordinary text. Interrupted input/generation retains `usage_confirmed: false`, the model, price reference and exact price snapshot. An absent UsageLog records uncertainty, not confirmed debt: it does not permanently prevent the key from starting another session. New connections remain subject to concurrency admission and confirmed-consumption quotas. Unknown consumption is not a zero-cost invoice.

A directly authenticated instance owner can mark terminal, unpriced audit records as reviewed through `POST /admin/realtime/reconcile`, using the normal admin JWT in the Authorization header. Send `{"apiKeyID":123,"requestIDs":[456],"note":"Provider consumption checked; no confirmed billable usage available"}`. Omit `requestIDs` to review up to 500 matching records for the specified key. The response contains `reviewedCount` and `usageOrBalanceChanged: false`; this operation adds reviewer/time/note metadata only, and never creates a UsageLog, fabricates zero consumption, or changes a balance. It is safe to retry. API keys, plugins and non-owner users cannot perform this operation. Automated provider reconciliation and a dedicated editor remain unavailable.

Before closing a continuously streaming VAD client, stop capture, send `input_audio_buffer.clear`, and wait for `input_audio_buffer.cleared`. Only provider-confirmed clearing of unused input cancels that pending reservation; closing the socket does not erase unknown consumption.

OpenAI independent input transcription (`session.audio.input.transcription`) has a separate usage stream and price card and remains disabled. The gateway removes SDK-provided transcription settings before forwarding session updates. A matching `session.model` is accepted and removed before forwarding because the model was selected when the connection opened. Realtime model audio understanding and output-audio transcripts remain available. Qwen's declared built-in input transcription is handled by its separate adapter.

Request-count quotas are checked against the existing confirmed UsageLog count when reserving a new audit record. Running token/cost checks exclude the already-reserved request count, so reaching that count does not cancel its own in-flight response. Usage details that fail validation still enter reconciliation; a missing price alone does not stop the session, and that modality is omitted from billed cost. Cost enforcement uses confirmed consumption rather than a provider-side hard prepaid spending lock.

Future work: independent transcription billing, automated provider reconciliation and a reconciliation management page, scoped browser ephemeral tokens, and verified MCP/search/video/multichannel support. These are not part of the verified first-release capability set.

Processing reservations left by a crash or failed database finalization remain available for investigation; an orphaned audit is not proof of debt and does not permanently block the key. Session concurrency is bounded by admission leases, shared across instances when Redis is configured and local otherwise. Audit metadata is written atomically to Request, independently of media/body storage switches.

If a Qwen3.5 response mixes speech and function-call text but provides only aggregate text tokens, the free-transcript/tool-argument split cannot be proven. Observed tokens are retained for reconciliation rather than guessed. Configure tool turns for text output, or prefer Qwen3.8/OpenAI for this case.

A channel retains one credential pool. Its HTTP configuration and channel-owned Realtime configuration must use matching official credentials and regions; use separate channels across regions or credential domains. Relay credentials are not official OpenAI credentials. Both channel and per-key tests perform actual WebSocket probes.

## Native fidelity and the safety boundary

The OpenAI native adapter returns each valid JSON message byte for byte, including error messages. The gateway adds an event ID to a client session.update, response.create, input_audio_buffer.clear, or input_audio_buffer.commit when its ID is missing or empty, so upstream rejections can be matched to the client event. The gateway also redacts upstream error and status_details.error details. WebSocket transport preserves JSON message boundaries, not physical RFC6455 fragmentation or TCP packet boundaries.

## OpenAI Realtime Translation

Enable Realtime on an existing OpenAI channel, add the upstream model `gpt-realtime-translate`, and optionally set a positive usage price per minute under **Realtime Translation input audio samples**. Without a rate, the gateway retains usage without recording a charge. Connect with a gateway API key to `/v1/realtime/translations?model=gpt-realtime-translate`. Set `session.audio.output.language` using `session.update`, wait for `session.updated`, then stream 24 kHz mono PCM16 through `session.input_audio_buffer.append`. Send `session.close` and read the remaining translated audio and transcript until `session.closed`.

The upstream session events provide no invoice usage. The gateway bills input PCM samples successfully written to the provider using the frozen per-minute rate, including audio sent before a disconnect. The audit identifies the measurement as `gateway_sent_pcm`, and the cost item quantity is the sent sample count. The standard conversation route `/v1/realtime` does not accept the translation model.
