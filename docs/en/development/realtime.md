# Realtime Gateway

Realtime uses an independent bidirectional WebSocket session, separate from Chat, Responses WebSocket, TTS and STT. The development branch is `feat/qwen-realtime`.

## Verification and release gate

Use the actual evidence recorded in `.agents/summary/realtime-delivery.md`. Mock tests establish local behavior, not official account availability, audio quality or provider billing accuracy. Every supported model needs its own live acceptance result.

The environment's OpenAI credential belongs to a third-party relay and must not be sent to OpenAI's official host. No Bailian key or Workspace is available. Official live acceptance for both OpenAI models and all Qwen models remains unexecuted and blocks production approval. The deployed reverse proxy, Redis failover behavior, audio intelligibility/duration and actual provider invoices also require acceptance.

## Configure through the admin UI

1. Use the existing `openai` or `bailian` channel and its credential manager.
2. Add an explicit `openai/realtime-ws` or `qwen/realtime-ws` endpoint. Each endpoint may set a 1000–60000 ms handshake timeout; leave it empty to inherit the server setting. Removing this endpoint disables new Realtime sessions. Keep the channel's HTTP Base URL intact.
3. OpenAI uses `wss://api.openai.com` with `/v1/realtime`. The initial model set is `gpt-realtime-2.1` and `gpt-realtime-2.1-mini`; deprecated models are not new defaults.
4. Bailian uses `wss://{WorkspaceId}.{region}.maas.aliyuncs.com` and `/api-ws/v1/realtime`. Select `cn-beijing` or `ap-southeast-1`; Workspace, key and region must match. Supported models are `qwen3.8-omni-flash-realtime`, `qwen3.5-omni-plus-realtime` and `qwen3.5-omni-flash-realtime`.
5. Configure supported models, mappings, model associations and API Key Profile permissions. A per-model protocol override can pin the model to the realtime endpoint for channel testing.
6. Configure the independent realtime price items using the current official rate and the installation's currency. Do not substitute historical text-model price codes or mix USD and CNY.
7. Channel testing verifies an actual WebSocket handshake, configuration acknowledgement and a lightweight conversation-item acknowledgement, without starting model generation. It does not certify audio or invoices.

## Client contract

Connect to `GET /v1/realtime?model={gateway-model-or-alias}` using WebSocket Upgrade and `Authorization: Bearer {AxonHub API Key}`. The first release supports server clients only. Browser Origin headers, URL credentials and client-selected upstream hosts are rejected. Authentication never falls back to No-Auth.

The optional `/v1/realtime/qwen?model={model}` entry uses Qwen event names and the same authentication, routing, quota, price and connection checks. It can only select Bailian realtime endpoints and still validates the implemented safe capability subset.

For OpenAI, valid GA events, extension fields and event order are forwarded without unnecessary JSON rewriting or resampling. No `OpenAI-Beta: realtime=v1` header is sent. When `session.created` contains the complete effective GA audio configuration, its confirmed defaults can be used directly. Configuration changes require `session.update` and its `session.updated` acknowledgement before using the new parameters. Session model changes, credential overrides and caller-supplied safety identity are prohibited. The upstream Safety Identifier is a one-way digest derived from the authenticated Gateway key and user/project identity.

Qwen has its own adapter. GA text events use `response.output_text.delta/done`, audio uses `response.output_audio.delta/done`, and audio transcripts use `response.output_audio_transcript.delta/done`. Function call IDs, argument deltas, final arguments and results remain associated. Unsupported fields are rejected explicitly.

Remote MCP, video, multichannel audio, browser ephemeral tokens and Preview compatibility are not default first-release capabilities. Upstream acceptance of a field is not evidence of Gateway support. Native OpenAI image usage requires complete image/cache price and usage breakdowns.

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

Qwen3.8 bills speech and its transcript separately. Qwen3.5's model billing policy preserves actual speech-transcript tokens while setting their billable text quantity to zero; text-only and function-argument output still use the text rate. Missing required prices reject connection establishment. Session prices are frozen so reloading or disabling a channel cannot lose confirmed usage.

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

Use the existing YAML/environment configuration loader, for example `AXONHUB_SERVER_REALTIME_MAX_DURATION=15m` with the prefix used by your deployment. No separate hidden feature flag is required; the explicit endpoint enables the capability.

The existing `cache.redis` configuration enables shared admission, connected lazily on the first realtime request. One same-slot Lua script atomically acquires global/project/key/channel concurrency and per-key connection-rate admission. Leases outlive the session hard deadline and expire after a crash. Unavailable Redis rejects new sessions. Without Redis, limits apply to one process only; multiple instances must share Redis and identical limits. Redis persistence and failover write-loss risk are deployment acceptance concerns.

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

Back up the database and channel configuration. Enable one restricted project with low concurrency, short duration and small quotas, then compare provider invoices before expanding. To roll back, remove realtime endpoints or disable the relevant channels, wait for bounded sessions or stop the server to cancel them, then revert the application. No database table migration is introduced, but remove new endpoint/price configuration before running older code that may not understand the new price enums. Retain confirmed invoice audit records.

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

Unknown usage is never estimated by modality ratios or billed as ordinary text. Interrupted input/generation retains `usage_confirmed: false`, the model, price reference and exact price snapshot. New realtime sessions for that API key return HTTP 409 until an administrator reconciles provider consumption and records confirmed UsageLog data. There is no automated reconciliation or dedicated reconciliation editor in this release. Unknown consumption is not a zero-cost invoice.

Before closing a continuously streaming VAD client, stop capture, send `input_audio_buffer.clear`, and wait for `input_audio_buffer.cleared`. Only provider-confirmed clearing of unused input cancels that pending reservation; closing the socket does not erase unknown consumption.

OpenAI independent input transcription (`session.audio.input.transcription`) has a separate usage stream and price card and is explicitly disabled until it is metered. Realtime model audio understanding and output-audio transcripts remain available. Qwen's declared built-in input transcription is handled by its separate adapter.

Request-count quotas are checked against the existing confirmed UsageLog count when reserving a new audit record. Running token/cost checks exclude the already-reserved request count, so reaching that count does not cancel its own in-flight response. Missing cache breakdowns or a used modality without a rate stop the session for reconciliation. Cost enforcement uses confirmed consumption rather than a provider-side hard prepaid spending lock.

Future work: independent transcription billing, automated/manual reconciliation tooling, scoped browser ephemeral tokens, and verified MCP/search/video/multichannel support. These are not part of the verified first-release capability set.

Processing reservations left by a crash or failed database finalization are checked against their owning session lease. Orphaned reservations require reconciliation. Audit metadata is written atomically to Request, independently of media/body storage switches. Ownership checks are shared across instances when Redis is configured and local otherwise.

If a Qwen3.5 response mixes speech and function-call text but provides only aggregate text tokens, the free-transcript/tool-argument split cannot be proven. Observed tokens are retained for reconciliation rather than guessed. Configure tool turns for text output, or prefer Qwen3.8/OpenAI for this case.

A channel retains one credential pool. Its HTTP and realtime endpoints must use matching official credentials and regions; use separate channels across regions or credential domains. Relay credentials are not official OpenAI credentials. Both channel and per-key tests perform actual WebSocket probes.
