# Plugins

---

An AxonHub plugin is a `.axplugin` zip. An owner uploads it under **Plugins** (`/plugins`), grants a subset of the manifest, and enables it. This guide describes the revised, unpublished contract. Hook signatures, registration, and return handling are implemented in the runtime and example scripts; see the [implementation status](../../solutions/plugin-system/README.md#9-任务管理). pluginApiVersion remains 1.

## What a plugin can do

- **Events.** Export `onEvent` and list any of the eight open events in `events`.
- **Request hooks.** `onRequest` runs once per request, before quota enforcement, model permission checks, model mapping, and routing. `onAttempt` receives the target protocol JSON body on each matching attempt, after protocol conversion, pass-through, and body adaptation, before channel overrides. A change from `onAttempt` applies only to that attempt.
- **Server middleware.** `server.middleware` handles gateway HTTP requests after API key authentication and before thread and trace resolution.
- **Pages.** Each `ui.pages` entry is a sidebar item and a full-page iframe.
- **Plugin KV.** `ctx.kv` and the page GraphQL KV fields read and write the same rows. Keys belong to this installation.
- **Host-proxied fetch.** `ctx.fetch` in `plugin.js` and `axonhub.fetch` in a page send HTTP through the host. The URL must match a granted `network` rule.

`onRequest` may change `model`. The model permission check and model mapping that follow use the new model. Both request hooks leave `stream` equal to the input. `onAttempt` also leaves `model` equal to the input for that attempt.

The eight events:

| Event | When it is published |
| --- | --- |
| `channel.auto_enabled` | A channel is enabled again automatically |
| `channel.credential_disabled` | An upstream credential is disabled |
| `channel.credential_enabled` | An upstream credential is enabled |
| `request.attempt_failed` | One upstream attempt fails |
| `request.failover` | Routing switches away from the current channel |
| `request.failed` | The request fails after its attempts |
| `request.completed` | The request succeeds |
| `api_key.quota_exceeded` | An API key exceeds its quota |

The event object uses camelCase. Credential events carry a fingerprint and a mask, never the credential text. Events do not include the request body or the response body. Delivery is at most once. A plugin does not receive an event it caused. `onEvent`'s return value is not applied; a throw or a timeout drops that delivery and does not retry it.

These four names belong to the existing webhook notifier and are rejected in `events`: `channel.auto_disabled`, `provider.quota.warning`, `provider.quota.exhausted`, `provider.quota.recovered`.

### Event payloads

`event` in `onEvent(ctx, event)` is a plain JS object. Every event has these fields:

| Field | Type and meaning |
| --- | --- |
| `id` | string, event UUID |
| `type` | string, one of the event names above |
| `severity` | string; current events use `info`, `warning`, or `error`, as listed below |
| `occurredAt` | string, UTC RFC 3339 timestamp |
| `subject` | object containing related entities; unavailable members are omitted, not set to `null` |
| `data` | object containing event-specific data; `{}` when there is none |
| `origin` | string; `""` for ordinary business events, `"plugin:<plugin id>"` for plugin-caused events |
| `depth` | number; `0` for ordinary business events, incremented for subsequent plugin-caused events |

The shapes of `subject` members are below. Every `id` here is numeric, and `model` is a string; these IDs are not GraphQL global IDs.

| Member | Shape |
| --- | --- |
| `channel` | `{ id, name, type, baseUrl, status }`; all fields except id are strings |
| `credential` | `{ fingerprint, masked }`, both strings; an ordinary key's fingerprint is the first 12 hex characters of its SHA-256 hash; both values are `"oauth"` for OAuth |
| `model` | Model name; omitted when unavailable |
| `apiKey` | `{ id, name }`, with a string name |
| `project` | `{ id }` |
| `request` | `{ id }` |

The subject column lists members that may be provided. A member can be absent when the request has not been persisted, there is no API key (for example, Playground), or no upstream credential was obtained. Check for a member before reading its fields.

| `type` | `severity` | `subject` members | `data` fields and types |
| --- | --- | --- | --- |
| `channel.auto_enabled` | `info` | `channel` | `{}` |
| `channel.credential_disabled` | `warning` | `channel`, `credential` | `errorCode: number`, `reason: string`, `expiresAt: string or null` (UTC RFC 3339; null means no expiry) |
| `channel.credential_enabled` | `info` | `channel`, `credential` | `trigger: "manual" or "expired"` |
| `request.attempt_failed` | `warning` | `channel`, `credential`, `model`, `apiKey`, `project`, `request` | `attempt: number`, `statusCode: number`, `errorType: string`, `errorMessage: string` |
| `request.failover` | `warning` | `channel` (being left), `model`, `apiKey`, `project`, `request` | `toChannelId: number`, `toChannelName: string`, `attempt: number` (the attempt being left) |
| `request.failed` | `error` | `channel` (last channel), `model`, `apiKey`, `project`, `request` | `statusCode: number`, `errorType: string`, `errorMessage: string`, `attempts: number` |
| `request.completed` | `info` | `channel`, `model`, `apiKey`, `project`, `request` | `latencyMs: number`, `promptTokens: number`, `completionTokens: number`, `attempts: number`, `stream: boolean` |
| `api_key.quota_exceeded` | `warning` | `apiKey`, `project`, `model`, `request` | `profile: string` (quota profile name), `window: { start: string or null, end: string or null, endInclusive: boolean }` |

`attempt` is the attempt number; `attempts` is the attempt count. An unavailable count is `0`. `statusCode` is `0` when no HTTP status is available; `errorType` is `""` when unavailable. Request error messages are truncated to 512 UTF-8 bytes. Token counts are `0` when usage is unavailable, which does not prove that no tokens were consumed. Quota window start/end values are UTC RFC 3339 timestamps; `null` means that side is unbounded, and endInclusive indicates whether the end boundary is included.

```js
export function onEvent(ctx, event) {
  if (event.type !== "request.failover") return;
  const channel = event.subject.channel;
  if (!channel) return;
  ctx.log.info("request failover", {
    eventId: event.id,
    fromChannelId: channel.id,
    toChannelId: event.data.toChannelId,
    attempt: event.data.attempt,
  });
}
```

## Closed capabilities

These names are rejected or absent. They have no author API in this version:

- Response hooks (`onResponse`, `onStreamChunk`)
- Dashboard slots
- Custom channel providers
- Project, API-key, or model scope on request hooks
- Builtin plugins
- `ctx.models`
- Package signatures
- Browser connections to external URLs

## Package layout

A `.axplugin` file is a zip:

```text
manifest.json
plugin.js          optional; required when events, hooks, or server.middleware are declared
ui/**              html, js, css, svg, png, woff2
```

A page-only plugin omits `plugin.js`. Page scripts run in the browser. They are not backend hooks.

`plugin.js` may import a relative path that stays inside the package, for example `import { failoverKey } from "./lib/key.js"`. The packer and the installer accept `manifest.json`, files under `ui/`, and the static import closure of `plugin.js`. `node:` specifiers and bare npm module names are rejected. Bundle npm dependencies into the package yourself. `eval` and dynamic code generation are disabled. A top-level `await` fails installation. Hook exports may be async.

## Packing

The packer is the standalone module `github.com/looplj/axonhub/cmd/pack-plugin`. It depends only on moejs. It copies the server's zip read and static-import compile, and it does not execute the plugin. Install still checks the manifest, the AxonHub version range, and named exports, so a zip that packs can still be rejected at install.

Install the packer from the `unstable` branch:

```bash
go install github.com/looplj/axonhub/cmd/pack-plugin@unstable
pack-plugin <plugin-directory> -o <outside>.axplugin
```

`@unstable` selects the current commit on that branch. Go records it as a pseudo-version `v0.0.0-<timestamp>-<commit>`. After a `cmd/pack-plugin/vX.Y.Z` tag is published, `@latest` selects the newest of those subdirectory tags. An AxonHub root tag `vX.Y.Z` does not version this module. Install the matching `cmd/pack-plugin/vX.Y.Z` when the checks should match a specific AxonHub release.

From this repository:

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/<name>
```

The default output is `<directory-name>.axplugin` next to the plugin directory. The output must be outside that directory; a path inside it fails. `*.axplugin` is gitignored. The package contains only `manifest.json`, files under `ui/`, root `plugin.js`, and files that `plugin.js` reaches through static relative imports. Every other file, such as `README.md`, `.git`, `.github`, `.claude`, `node_modules`, tests, and `package.json`, is left out, and the command prints the skipped paths. A directory that holds no packed file is listed once, for example `.git/`. Files loaded only through dynamic `import()` are not collected, so check the skipped list before you upload. A symlink fails the pack only when it would be packed: `manifest.json`, `plugin.js`, anything under `ui/`, or an imported file.

Upload that zip on `/plugins` to try it. Publishing it on the official index is a separate step, below.

## manifest.json

Fields use camelCase. A field that is not open is rejected at install and is not stored. That includes `hooks.response`, `hooks.streamChunk`, `ui.panels`, `providers`, the permission name `fetch`, and any unknown field.

| Field | Required | Rule |
| --- | --- | --- |
| `schemaVersion` | yes | `1` |
| `id` | yes | `^[a-z0-9]+(\.[a-z0-9-]+)+$`, at most 64 characters. Immutable after install. `axonhub.webhook` is reserved |
| `name` | yes | Non-empty string |
| `version` | yes | Semantic version |
| `description` | no | String |
| `axonhubVersion` | yes | npm-style version range. Prefer `^1.0.0`. A prerelease suffix on the running AxonHub version is removed before the check, so `v1.0.0-beta9` satisfies `^1.0.0` |
| `pluginApiVersion` | yes | `1` |
| `permissions` | no | Names from the list below. Omitted means `[]` |
| `network` | no | At most 32 rules. Omitted means `[]` |
| `tasks` | No | See scheduled tasks below |
| `events` | no | Names from the eight events above. A non-empty list requires `export function onEvent` |
| `hooks.request` | no | Object: enabled is required; true requires non-empty requestTypes and an onRequest export; false keeps configuration without running it |
| `hooks.attempt` | no | Object: enabled is required; true requires non-empty apiFormats and an onAttempt export; false keeps configuration without running it |
| `server.middleware` | no | Array of `{ "handler": "namedExport" }` objects, in execution order. Each handler must be a distinct function exported from `plugin.js` |
| `configSchema` | no | JSON Schema object, described below |
| `ui.pages` | no | Sidebar pages, described below |
| `ui.actions` | no | Global toolbar actions that open plugin dialogs |

Open permissions: `read_channels`, `read_requests`, `read_dashboard`, `read_api_keys`, `read_projects`, `read_settings`, `write_channels`, `webhook`, `sql`. Outbound URLs go in `network`. `read_plugins` and `write_plugins` are user scopes, not plugin permissions. KV is not a permission.

A `network` entry is either an exact `https` URL or `{ "regex": "^https://api\\.example\\.com/v1/items/[0-9]+$" }`. An exact URL includes a path and no userinfo, query, or fragment, and it matches that path only. A regex is a full match, at most 256 characters, against the URL with query and fragment removed. The request may still carry a query. Redirects are not followed: statuses 301, 302, 303, 307, and 308 reject both `ctx.fetch` and `axonhub.fetch` Promises with `INVALID_STATE`.

`configSchema.type` is `object`. Each property is `string`, `integer`, `number`, `boolean`, or `array`, and may set `enum`, `minimum`, `maximum`, `default`, `title`, `description`, and `format: "secret"`. An `array` property must set `"items": {"type": "string"}` and holds a list of strings (for example `{"type": "array", "items": {"type": "string"}, "default": ["gpt-4o"]}`); it does not accept `enum` or `format`. The settings form shows it as a list with add and remove buttons. `configSchema.required` lists property names the plugin needs. A new install starts disabled; the owner can save partial config while it is disabled, but enabling it, saving config while it is enabled, or upgrading it while it is enabled fails until every required field has a value or a default (an empty string or an empty array counts as missing). Unknown fields are rejected. `ctx.config` is the saved object, including secret text. A field the owner has not saved is absent, so apply the schema default in the script. The settings form shows those defaults. GraphQL and the page receive `******` for a stored secret; sending that mask back leaves the stored value unchanged.

`ui.pages[]` requires `key`, `entry`, and `title`. `key` matches `^[a-z0-9][a-z0-9-]{0,31}$` and is unique in the plugin. `entry` is an existing `.html` file under `ui/`. `title` is a string, or a language map that includes a non-empty `en`. The sidebar prefers the current language, then `en`, then `key`.

## plugin.js

The host calls named exports only. `export default` is not scanned, and there is no register function. An enabled hook must be a function export. An exported function that the manifest does not declare is not called.

All entry points put ctx first: `onEvent(ctx, event)`, `onRequest(ctx, request)`, and `onAttempt(ctx, body)`.

onRequest matches RequestType and receives the unified request's public fields as a JS object with snake_case keys. The host preserves internal bookkeeping and non-public audio/image data outside replace/patch. onAttempt matches the actual outbound APIFormat of this attempt and receives the target protocol's JSON body object, not llm.Request, a string, or bytes. Multipart, binary, and non-object JSON bodies skip attempt hooks.

```json
{
  "hooks": {
    "request": { "enabled": true, "requestTypes": ["chat"] },
    "attempt": { "enabled": true, "apiFormats": ["openai/chat_completions"] }
  }
}
```

A declared hook requires enabled. When true, its matching list must be non-empty. When false, its scope may remain configured. Omitting the hook object means it is undeclared; an empty list never means “all,” and list contents do not determine enabled. Scope values must be exact members of the lists below; unknown values, duplicates, and wildcards are rejected. The installation's enabled remains the plugin-wide switch. The hooks match independently; retries rematch attempt hooks against the actual outbound format even if onRequest did not run.

`requestTypes`: `chat`, `embedding`, `rerank`, `image`, `video`, `compact`, `completion`, `speech`, `transcription`, `translation`, `moderation`, `alpha_search`, `systemone`, `decisions`.

`apiFormats`: `openai/chat_completions`, `openai/completions`, `openai/responses`, `openai/responses-ws`, `openai/responses_compact`, `openai/image_generation`, `openai/image_edit`, `openai/image_variation`, `openai/embeddings`, `openai/video`, `openai/audio_speech`, `openai/audio_transcriptions`, `openai/audio_translations`, `openai/moderations`, `openai/alpha_search`, `gemini/contents`, `anthropic/messages`, `aisdk/text`, `aisdk/datastream`, `gemini/embeddings`, `jina/rerank`, `jina/embeddings`, `typesafe/systemone`, `openai/decisions`, `ollama/chat`, `seedance/video`, `zenmux/video`, `modelscope/image_generation`.

`onRequest` receives the public JSON. The host removes `raw_request`, `request_type`, `api_format`, `transform_options`, `transformer_metadata`, and provider extensions. Read the request type and protocol from `ctx.request`. Bytes tagged `json:"-"` are absent, including image bytes, audio bytes, and the alpha search raw body. Text usually lives here:

| Type | Public fields |
| --- | --- |
| `chat` | `messages` |
| `embedding` | `embedding.input`. Strings are present; token arrays have no text |
| `rerank` | `rerank.query`, `rerank.documents` |
| `image` | `image.prompt` |
| `video` | `text` on `video.content` items whose `type` is `text` |
| `compact` | `compact.instructions`, `compact.input` |
| `completion` | `completion.prompt`, `completion.suffix` |
| `speech` | `speech.input`, `speech.instructions` |
| `transcription` | `transcription.prompt` |
| `translation` | `translation.prompt` |
| `moderation` | `moderation_request.input`: a string, an array of strings, or `text` on multimodal parts |
| `alpha_search` | The raw body is not in the public JSON |
| `systemone` | `systemone.state` |
| `decisions` | `decisions_request.input` |

The host resolves the outbound format once: the HTTP request’s APIFormat takes precedence, followed by the candidate format and the transformer default. Hook matching and ctx.attempt.apiFormat use that same result. A skipped format match produces a Debug log with the plugin, channel, format, and reason.

| Return or call | Result |
| --- | --- |
| JSON object | replace: replaces the whole public object; omitted fields are removed, not merged |
| Array | patch: an RFC 6902 JSON Patch operation list; an empty array succeeds without edits |
| undefined or no return | Keeps the pre-call object and ignores in-place edits |
| null, scalar, invalid return, throw, or timeout | Keeps the original, drops this call's state edits, and counts one failure |
| ctx.reject(400 / 403 / 429, message) | onRequest only; stops the request with a non-empty message; a business rejection, not a failure |

Patches support add/remove/replace/move/copy/test from [RFC 6902](https://www.rfc-editor.org/rfc/rfc6902). Paths use [RFC 6901 JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901), such as `/messages/0/content` or `/messages/-` to append. Apply to the pre-call object: any failed operation or non-object final result rolls back the entire patch. Both replace and patch pass type and protected-field validation before the host commits the request and state. The next plugin sees the committed result.

onRequest may change model, followed by permission checks and mapping. For speech requests, onRequest must also preserve `speech.stream_format`, including whether it is omitted, because the host selects the response writer before hooks run. Neither hook may change streaming semantics; onAttempt must also preserve the selected model, including through deletion or root replacement. The host validates protocol-specific body and endpoint/WS controls. Protected-field violations discard the rewrite and state without counting a failure. Catching ctx.reject still rejects the request.

Protection rules are specific to the outbound APIFormat. For example, omitted stream means false for ordinary chat, but true for Ollama; type/generate are frame controls only for Responses WebSocket, and stream_format is protected for speech. A rejected rewrite increments protectedDrops once per call, shown on the plugin card and detail view. The host emits a Warn with plugin, hook, request ID, format, field names, and reason, without field values. Request edits and state are discarded; the consecutive-failure count is unchanged.

```js
export function onEvent(ctx, event) {
  ctx.log.info(event.type);
}

export function onRequest(ctx, request) {
  if (request.model === "blocked-model") ctx.reject(403, "model is disabled");
  return { ...request, temperature: 0.2 }; // replace
}

export function onAttempt(ctx, body) {
  return [{ op: "add", path: "/temperature", value: 0.2 }]; // patch
}
```

Plugins run in ascending priority; new installations default to 100. Each attempt regenerates its upstream request without retaining body edits from a previous attempt. Attempt hooks never write back to llm.Request or trigger whole-request deep copies. Private upload buffers remain host-owned and are retained when onRequest commits its public fields.

Pass-through can overwrite non-model edits from onRequest, recorded in overriddenByPassThrough and a warning. onAttempt runs after pass-through, so its replace/patch takes effect. Channel overrides run next and win on matching fields. Editing a pass-through body with attempt does not require disabling pass-through.

ctx.request uses camelCase: projectId, apiKeyId, model, requestType, and apiFormat, captured at middleware entry; Playground uses a null apiKeyId. ctx.attempt exists only on onAttempt and includes channel (id, name, type), the actual model, and outbound apiFormat, with no credentials. The body need not contain model; read ctx.attempt.model instead. This interface does not expose URL, method, HTTP header, or credential edits.

## Managed tables

Declare tables in `manifest.json` and request the `tables` permission. AxonHub
creates the physical tables on enable and resolves their names from the current
installation. The same manifest ID resumes its existing data automatically after
reinstall. Uninstall retains managed table data by default. Select **Also
permanently delete managed table data** to clear it.

Physical names use the `axp_` prefix, for example `axp_42_daily_usage`.
Plugin code uses the declared name `daily_usage`. On MySQL and TiDB, table
migrations and purges require at least two primary database connections.

```json
{
  "permissions": ["tables"],
  "tables": {
    "daily_usage": {
      "columns": {
        "day": { "type": "string", "maxLength": 10 },
        "requests": { "type": "integer" }
      },
      "indexes": [{ "columns": ["day"], "unique": true }]
    }
  }
}
```

```js
const usage = ctx.tables.open("daily_usage")
usage.upsert({ conflict: ["day"], row: { day: "2026-11-23", requests: 42 } })
const page = usage.list({
  where: [{ column: "day", op: "gte", value: "2026-11-01" }],
  orderBy: [{ column: "day", direction: "desc" }],
  first: 30
})
```

- Each row has a host-assigned integer `id`. Use `insert(row)`, `get(id)`,
  `update(id, patch)`, `delete(id)` or `upsert({ conflict, row })`. Upsert's
  conflict columns must match a declared unique index in order. Get and update
  return `null` for missing rows, and delete returns a boolean.
- `list({ where, orderBy, first, after })` returns `{ rows, nextCursor }`. Pass
  the cursor back with the same filters and ordering. Filters are combined with
  AND and support `eq`, `lt`, `lte`, `gt`, `gte`. Ordering uses non-nullable
  columns and adds the primary key to break ties. Pages default to 50 rows and
  allow up to 100.
- Column types are `string`, `text`, `integer`, `number` and `boolean`.
  Strings require `maxLength`. Columns are required unless declared `nullable`.
  Integers outside JavaScript's safe range use decimal strings. An upgrade can
  add tables, nullable columns and ordinary indexes. Unique indexes can be
  declared on new tables.
- Tables are accessible with `tables` alone. Raw `sql` keeps its existing
  full-database access. The managed API bounds argument size, result size, write
  rate and execution time.

Plugin pages can open the same managed table with `window.axonhub.tables.open("daily_usage")`.
Method names, arguments, and result values match `ctx.tables`, with each page operation returning a Promise.
The plugin must have the `tables` grant, and the caller must be an Owner or have `write_plugins`.
See the [to-do list example](../../../plugins/examples/table-todos/README.md) for the page flow.

## Scheduled tasks and dynamic SQL

A manifest can declare scheduled tasks and the `sql` permission. This example shows the two fields and their JavaScript entry point.

```json
{
  "permissions": ["sql"],
  "tasks": {
    "daily-count": {
      "schedule": { "cron": "0 2 * * *", "timezone": "Asia/Shanghai" }
    }
  }
}
```

```js
export function onTask(ctx, run) {
  if (run.key !== "daily-count") return;
  const end = run.scheduledAt ?? new Date().toISOString();
  const start = new Date(Date.parse(end) - 86_400_000).toISOString();
  const placeholder = ctx.sql.dialect === "postgres" ? "$1" : "?";
  const { rows } = ctx.sql.query(
    "SELECT COUNT(*) AS total FROM requests WHERE created_at >= " + placeholder,
    [start]
  );
  ctx.log.info("Request count", { total: rows[0][0] });
}
```

### Scheduled tasks

A plugin can declare up to 16 tasks, keyed by task ID. Each `schedule` requires a five-field `cron` expression and accepts an optional IANA `timezone` (UTC by default). The host calls `onTask(ctx, { key, runId, trigger, scheduledAt })`. The trigger is `scheduled` or `manual`. Manual runs receive `null` for `scheduledAt`.

The plugin details show the next trigger and the last 20 runs. Owners can pause automatic triggers, change the schedule, and run a task on demand. Instances share the database claim record. The scheduler claims a slot at most once and skips missed minutes or ticks while an earlier run is active. A task has a 30-second deadline. A running task can finish across upgrades while it remains declared and the plugin stays active. Disabling the plugin or removing the task cancels its run. Unfinished runs show an unknown outcome after the instance exits and its lease expires.

Upgrade every replica before installing a plugin that declares `tasks`.

### Dynamic SQL

An Owner can grant `sql` to give `plugin.js` **all SQL privileges of the AxonHub primary database account**, including reads, writes, and DDL. This includes access to credentials and metadata visible to that account. Owner access is required to configure a plugin with this grant. The host checks the current activation status and grant before each call.

`ctx.sql.query(statement, params?)` returns `{ columns, rows }`, with rows represented by arrays in column order. `ctx.sql.exec(statement, params?)` returns `{ rowsAffected }`. Both methods use the same grant and accept SQL generated at runtime. Bound values can be numbers, strings, booleans, or null. Integers beyond JavaScript’s safe range are decimal strings, and binary values are Base64 strings.

Use the database dialect and its native placeholders. Calls have a 10-second deadline, and query results are limited to 1,000 rows and 1 MiB. Each call obtains its own connection. Complete any transaction within one call.

## Server middleware

```json
{
  "server": {
    "middleware": [{ "handler": "resolveSessionTrace" }]
  }
}
```

```js
export function resolveSessionTrace(ctx, request) {
  const sessionId = request.headers["x-deepseek-harness-session-id"]?.[0]?.trim()
  if (sessionId) return { context: { axonhub: { traceId: sessionId } } }
}
```

Enabled handlers run in plugin priority order, then in manifest order within each plugin. The same gateway entry point covers OpenAI, Anthropic, Gemini, Codex, and WebSocket handshakes. Each handler receives `handler(ctx, request)`. The request snapshot contains `method`, `path`, `headers` (lowercase keys and string arrays), and `projectId` and `apiKeyId` (numbers or `null`). The path excludes the query. The host filters credential headers and leaves the body for the API handler.

Return `undefined` to continue, or an object with either of these fields:

- `headers` accepts `set` (header names to non-empty string arrays) and `remove` (an array of names). Names are case-insensitive. The host applies removals before replacements and protects credential, forwarding, and transport headers. Later handlers see the updated values.
- `context` maps namespaces to JSON objects. Write arbitrary JSON fields under your plugin ID, or supply `threadId` and `traceId` candidates under `axonhub`. IDs are trimmed, limited to 255 bytes, and cannot contain control characters. Your namespace merges by field, with later writes replacing the entire field. The host keeps the first non-empty candidate for each ID. Thread and trace resolution prefer explicit headers, and trace resolution also checks built-in client detection before the candidate. Later server handlers and LLM hooks can read the committed metadata through `ctx.requestContext` when present.

The host validates each result before applying it. Each result and the accumulated context have a 64 KiB limit. All handlers share a 25 ms budget, including VM acquisition. A throw, a rejected promise, an invalid result, or a timeout discards that handler's changes. When the budget expires, the request proceeds with the changes already applied. Call `ctx.reject(status, message)` with 400, 403, or 429 to terminate the request with a gateway JSON error. Handler-local `ctx.state` is discarded when the handler returns. Use the returned `context` to pass data to later hooks.

The running handler list is refreshed after plugin writes commit. A request that has acquired a plugin snapshot finishes this stage on that snapshot, including when the plugin is upgraded or disabled during execution.

## ctx

Use the `ctx` argument of the current call. Saving `ctx` or its methods on a module binding does not make them work on a later call.

| Member | Where it works |
| --- | --- |
| `ctx.config` | Saved config for this plugin. Read-only |
| `ctx.log.info` / `warn` / `error` | Plugin log. The second argument is a JSON object. Leave secrets and full request bodies out |
| `ctx.kv.get` / `set` / `delete` / `list` | This installation's KV. Values are UTF-8 strings of at most 256 KiB. A namespace or key matches `^[A-Za-z0-9_.:-]{1,128}$`; namespace defaults to `default`. One call may write 10 times, and one plugin may write 60 times per minute. `get` returns `null` when the key is missing or expired. `ttlSeconds` of `0` or omitted does not expire. Upgrade keeps the rows; installing the same id again after delete starts empty |
| `ctx.fetch(url, init)` | Host-proxied HTTPS. Returns a promise of `{ status, headers, body }`. Ordinary non-2xx responses resolve; the script reads `status`. Redirect statuses 301/302/303/307/308 reject with `INVALID_STATE` and are not followed. There is no global `fetch` |
| `ctx.request` | Request metadata, above |
| `ctx.requestContext` | Committed server middleware metadata, when present |
| `ctx.attempt` | `onAttempt` only |
| `ctx.state` | JSON object for this request and plugin. `onRequest` and `onAttempt` share committed state, including across VMs |
| `ctx.reject(status, message)` | `onRequest` and server middleware |

`ctx.state` starts as `{}`. The LLM request and attempt hooks commit it when the hook returns successfully, including `undefined`. A throw, a timeout, an illegal return, or `ctx.reject` drops this call's state edits. A KV write stays written.

These three are present only on `onEvent`:

- `ctx.counter.add(key, windowSeconds)` increments by one and returns the window count. `ctx.counter.get` reads it. The same plugin, key, and window are one counter. With a memory cache the count is per instance; with Redis the instances share it.
- `ctx.webhook.send(targetName, payload)` sends to an existing webhook target name. It does not take a URL. This requires the `webhook` grant. A string `payload.message` fills the template message.
- `ctx.axonhub.channels.disable(id, options)` disables that channel. This requires the `write_channels` grant. Disabling a channel that is already disabled succeeds.

Module-level variables survive VM reuse, and a VM can also be discarded after a timeout. Use them for constants and caches you can rebuild. Per-request data goes in `ctx.state`. Data that must survive a restart goes in `ctx.kv`.

```js
ctx.kv.set("cursor", "10", { ttlSeconds: 3600 });
const raw = ctx.kv.get("cursor");
```

### KV calls and pagination

These `ctx.kv` methods return synchronously and throw a `PluginHostError` with a `code` on failure:

| Call | Return value |
| --- | --- |
| `ctx.kv.get(key, { namespace }?)` | String value, or `null` when missing or expired |
| `ctx.kv.set(key, value, { namespace, ttlSeconds }?)` | `undefined`; value must be a string, so use `JSON.stringify` for JSON objects |
| `ctx.kv.delete(key, { namespace }?)` | boolean indicating whether a row was deleted |
| `ctx.kv.list({ namespace, prefix, first, after }?)` | The `KVPage` object below |

The options object and all its fields are optional. `namespace` defaults to `"default"`; `prefix` defaults to `""` and filters by key prefix; `first` is an integer from 1 through 100, defaulting to 100. `after` is the previous page's `nextCursor` string; omit it or use `""` for the first page. Do not pass the final page's `null` to `ctx.kv.list`.

```ts
type KVEntry = {
  namespace: string;
  key: string;
  value: string;
  updatedAt: string;
  expiresAt: string | null;
};
type KVPage = { entries: KVEntry[]; nextCursor: string | null };

// ctx.kv.list(options?) -> KVPage
// options: { namespace?: string; prefix?: string; first?: number; after?: string }
```

Results contain only unexpired entries, ordered by key ascending, excluding the key at the cursor. `nextCursor` identifies the next page and is `null` on the last page; an empty result has `entries: []`. `updatedAt` and non-null `expiresAt` are UTC RFC 3339 timestamps, possibly with fractional seconds. Keep namespace/prefix unchanged between pages. Each page reads current data; pagination does not provide a snapshot across pages.

```js
export function onEvent(ctx, event) {
  let after;
  do {
    const page = ctx.kv.list({ namespace: "default", prefix: "note.", first: 50, after });
    for (const entry of page.entries) {
      ctx.log.info("stored key", { key: entry.key, updatedAt: entry.updatedAt });
    }
    after = page.nextCursor;
  } while (after !== null);
}
```

Pages call `pluginKVList(namespace, prefix, first, after)` through `axonhub.graphql`. Its result also has `entries` and `nextCursor`, but fields must be explicitly selected in GraphQL. GraphQL accepts `after: null` for the first page. `pluginKV` returns a complete entry or `null`, `setPluginKV` returns an entry, and `deletePluginKV` returns a boolean; unlike `ctx.kv.get`, `pluginKV` does not return just the value string.

## Global toolbar actions

`ui.actions` registers actions in the top-right controls of the authenticated layout. The host renders buttons and only loads an iframe after a click. Desktop shows up to two buttons with the remainder in an overflow menu; mobile uses one menu.

```json
"ui": {
  "actions": [{
    "key": "notes",
    "slot": "app.header.actions",
    "title": { "en": "Quick notes", "zh-CN": "随手记" },
    "icon": "puzzle",
    "effect": { "type": "dialog", "entry": "ui/notes.html" }
  }]
}
```

The first version only accepts slot `app.header.actions` and effect type `dialog`. Keys, titles and entry paths follow page rules; action keys are unique within actions. The optional icon defaults to puzzle and accepts puzzle/search/activity/settings. Unknown fields, slots and effects are rejected. Panels and channel menu actions are not supported.

Dialogs reuse the same Bridge, resources and permission checks. Actions-only plugins can use GraphQL, KV and granted fetch rules. Initialization includes the action key, slot, `context: {}`, `page: null` and `panel: null`. Global actions always receive `projectId: null`; their requests do not inherit the selected project header. Ordinary pages retain their existing project context. Closing cancels pending requests; reopening creates an independent session. Navigation closes the dialog.

Adding an action requires upgrade preview confirmation for enabled plugins. See the packable [toolbar-notes example](../../../plugins/examples/toolbar-notes/README.md).

## Pages

Each page is an iframe in the authenticated layout. The sidebar entry is under **Plugins**. The route is `/plugins/<plugin id>/pages/<key>`. The iframe cannot read the host cookie or `localStorage`.

Write asset URLs relative to the file that contains them. A path that starts with `/` drops the plugin resource token. The host serves `bridge.js`; do not put it in the package. From `ui/notes/index.html` the script URL is `../../_axonhub/bridge.js`. From `ui/index.html` it is `../_axonhub/bridge.js`. CSS `url(...)` is relative to the CSS file. Ship built files with relative URLs (`base: './'` in Vite).

The page talks to `window.axonhub`. Call `axonhub.ready()` once, then `axonhub.graphql(query, variables)` and `axonhub.fetch(url, init)`. The host proxies both. GraphQL uses the intersection of the plugin grant and the signed-in user's permissions. `axonhub.fetch` uses the same granted `network` rules as `ctx.fetch`. The page CSP keeps `connect-src 'none'`, so the browser does not connect to an external URL. `axonhub.navigate` accepts a single-slash site path. `axonhub.toast` shows a text notice.

Page KV uses the plugin GraphQL fields `pluginKV`, `pluginKVList`, `setPluginKV`, and `deletePluginKV`. They address the same rows as `ctx.kv`. Anyone who can open the page can read. Writing requires the signed-in user to have `write_plugins`. `init.config` follows the settings form: schema defaults are filled in, and a stored secret is `******`.

### Page bridge API

Use `window.axonhub` after loading the host-provided `bridge.js`. `ready()` returns the initialization snapshot; `onInit` receives the same shape, while `onTheme` receives only the theme portion:

```ts
type ThemeSnapshot = {
  theme: "light" | "dark";
  colorScheme: string;
  tokens: Record<string, string>;
};
type InitSnapshot = ThemeSnapshot & {
  locale: string;
  projectId: number | null;
  page: string | null;
  panel: null;
  action: string | null;
  slot: "app.header.actions" | null;
  context: Record<string, unknown>;
  config: Record<string, unknown>;
  canWriteKV: boolean;
};
```

`locale` is the current UI language; `projectId` is the selected project's numeric ID, or null without a valid selection; `page` is the manifest page key. `config` includes defaults and masked secrets. `canWriteKV` indicates whether the current user can write plugin KV. Snapshots are deeply frozen and cannot be modified.

| Method | Parameters and return value |
| --- | --- |
| `ready()` | `Promise<InitSnapshot>`; starts the handshake; repeated calls return the same Promise |
| `onInit(handler)` | Handler receives `InitSnapshot`; returns an unsubscribe function `() => void` |
| `onTheme(handler)` | Handler receives `ThemeSnapshot`; returns an unsubscribe function `() => void` |
| `graphql(query, variables?, options?)` | A `Promise` of the complete GraphQL response `{ data?, errors?, extensions? }`, not just data |
| `navigate(path)` | `Promise<null>`; path is a single-slash site path with no `//`, backslashes, or control characters |
| `toast(message, options?)` | `Promise<null>`; message is non-empty plain text; options is `{ kind?: "info" / "success" / "warning" / "error" }`, default info |
| `fetch(url, init?)` | `Promise<{ status: number, headers: Record<string, string>, body: string }>`; not a browser Response object |

`onInit` runs once after a successful handshake. Subscribing after initialization delivers the initialization snapshot in the next microtask. `onTheme` delivers an initial theme and runs again on theme updates; subscribing after initialization also asynchronously delivers the current theme. Neither callback runs after it is unsubscribed. Registering a listener alone does not start the handshake; call `ready()` or another asynchronous method. Synchronous callback exceptions are reported as plugin page errors without stopping other listeners; asynchronous work started by a callback must handle its own rejections.

For `graphql`, query must be a non-empty string and variables a JSON object, defaulting to `{}`. Options is `{ operationName?: string }`; a query containing multiple operations requires a non-empty operationName. There is no direct `axonhub.kv` API; see the KV query example below.

For `fetch`, init is `{ method?, headers?, body? }`: method must be uppercase `GET`, `POST`, `PUT`, `PATCH`, or `DELETE`, defaulting to GET; headers is an object of string values, and body is a string. Response header names are lowercase and body is UTF-8 text; parse JSON yourself with `JSON.parse(result.body)`. Ordinary HTTP errors such as 404 and 500 still return a result; check result.status. **301, 302, 303, 307, and 308 are exceptions: the host neither follows the redirect nor returns that response, and rejects the Promise with `INVALID_STATE`.** The same redirect rule applies to `ctx.fetch`.

#### Return values and error handling

`graphql`, `navigate`, `toast`, and `fetch` automatically wait for ready before sending a call. The handshake times out after 10 seconds; a sent bridge call times out after 15 seconds. Successful `navigate` only means the host accepted navigation, not that the destination loaded. If navigation destroys the iframe, pending calls may end with `DISPOSED`.

A bridge failure rejects the Promise with an `Error` named `AxonhubBridgeError`, carrying `code`, `message`, and optionally a numeric `status`. Status is the host endpoint's HTTP status, not the fetch target's result.status. Display the message if needed, but branch on code.

| `code` | Meaning |
| --- | --- |
| `INVALID_ARGUMENT` | Bridge argument validation failed |
| `NOT_READY` | No usable handshake session |
| `UNAUTHENTICATED` / `FORBIDDEN` / `NOT_FOUND` / `RATE_LIMITED` | Host endpoint returned 401 / 403 / 404 / 429 respectively |
| `TIMEOUT` | Handshake, call, or proxied request timed out |
| `NETWORK_ERROR` | Network or proxy transport failure |
| `INVALID_STATE` | An outbound redirect listed above, an invalid UTF-8 response, or another state error |
| `DISPOSED` | Page or bridge session was destroyed |
| `INTERNAL` | Other host errors or invalid responses |

When a GraphQL HTTP request succeeds, the Promise resolves even if its response contains `errors`. Check `result.errors`, inspect `errors[].extensions.code` as needed, and decide whether to use partial data. These are separate from bridge failures caught by `catch`.

```js
const axonhub = window.axonhub;
const stopInit = axonhub.onInit((init) => {
  document.documentElement.lang = init.locale;
});
const stopTheme = axonhub.onTheme(({ theme }) => {
  document.documentElement.dataset.theme = theme;
});
window.addEventListener("pagehide", () => {
  stopInit();
  stopTheme();
}, { once: true });

async function readFirstPage() {
  try {
    const init = await axonhub.ready();
    const result = await axonhub.graphql(`query Notes($after: String) {
      pluginKVList(prefix: "note.", first: 20, after: $after) {
        entries { namespace key value updatedAt expiresAt }
        nextCursor
      }
    }`, { after: null });
    if (result.errors?.length) {
      console.error("GraphQL errors", result.errors);
      return;
    }
    console.log(init.canWriteKV, result.data.pluginKVList);
  } catch (error) {
    console.error("Bridge call failed", error.code, error.status, error.message);
  }
}
readFirstPage();
```

### Page themes

Plugin pages run in sandboxed iframes and ship their own stylesheets. The host supplies computed theme variables only; it does not provide components or the console's CSS files.

After the handshake and whenever the user changes the light/dark mode or color palette, `bridge.js` copies computed values from the parent page's `<html>` to the plugin page's `<html>`. It also sets `color-scheme` to match the current mode. Reference the same variable names in your CSS:

```css
body {
  background: var(--background, #f4f5f7);
  color: var(--foreground, #1c1f24);
  font-family: var(--font-sans, ui-sans-serif, system-ui, sans-serif);
}

code, .mono {
  font-family: var(--font-mono, ui-monospace, monospace);
}

.card {
  background: var(--card, #ffffff);
  color: var(--card-foreground, inherit);
  border: 1px solid var(--border, #e3e6eb);
  border-radius: var(--radius, 0.75rem);
  box-shadow: var(--shadow-sm);
}

button {
  background: var(--primary, #1c1f24);
  color: var(--primary-foreground, #ffffff);
  border-radius: var(--radius, 0.75rem);
}

button.secondary {
  background: var(--secondary, transparent);
  color: var(--secondary-foreground, inherit);
}

input, textarea {
  border: 1px solid var(--input, #e3e6eb);
  border-radius: var(--radius, 0.75rem);
}

input:focus-visible {
  outline: 2px solid var(--ring, transparent);
}

.popover {
  background: var(--popover, #ffffff);
  color: var(--popover-foreground, inherit);
  box-shadow: var(--shadow-md);
}
```

The second argument to `var()` applies before the handshake completes or when the current palette does not define that variable. Once the host supplies a value, the fallback is no longer used.

`--muted` is a surface color; use `--muted-foreground` for secondary text. `--accent` is also a surface color. Use `--primary` for primary buttons.

#### Transmitted variables

| Purpose | Variables |
| --- | --- |
| Page background and text | `--background`, `--foreground` |
| Cards | `--card`, `--card-foreground` |
| Popovers | `--popover`, `--popover-foreground` |
| Primary buttons | `--primary`, `--primary-foreground` |
| Secondary buttons | `--secondary`, `--secondary-foreground` |
| Muted surfaces and secondary text | `--muted`, `--muted-foreground` |
| Highlighted surfaces | `--accent`, `--accent-foreground` |
| Destructive actions | `--destructive`, `--destructive-foreground` |
| Borders, inputs, and focus rings | `--border`, `--input`, `--ring` |
| Corner radius | `--radius` |
| Font families | `--font-sans`, `--font-mono` |
| Shadows | `--shadow-2xs`, `--shadow-xs`, `--shadow-sm`, `--shadow`, `--shadow-md`, `--shadow-lg`, `--shadow-xl`, `--shadow-2xl` |

Only font family names are transmitted; font files are not loaded into the iframe. Keep generic fallback families in `font-family` for systems without the named fonts.

Sidebar, chart, table, letter-spacing, and spacing variables are not transmitted. They belong to the console's internal layout and are outside the plugin page contract.

Empty values and values containing control characters, `;`, `{}`, `<>`, or `\` are discarded. Allowlisted variables absent from a subsequent message are removed from the plugin page so values from the previous palette do not linger.

## Grants

The owner enables a plugin with a subset of `permissions` and a subset of `network`. Host APIs and page GraphQL check that granted set. Adding a permission, network rule, event, hook, or page to an enabled plugin asks the owner to confirm before the new package replaces the old one.

`write_*` grants are highlighted because the page can run the mutations in that scope. `write_channels` lets the page run channel mutations and read credential plaintext when the signed-in user also has that permission, including changing a channel URL. The page still cannot open its own connection. It can send data only through an already granted `axonhub.fetch` rule.

## Publishing

AxonHub reads the [community plugin index](https://raw.githubusercontent.com/looplj/axonhub-plugins/main/registry.json). The index updates independently of AxonHub releases.

For centralized packaging and release, submit the source under [`plugins/community/`](../../../plugins/community/) in AxonHub and bump the plugin version in `manifest.json`. After the change merges to `unstable`, AxonHub sends a `community-plugins` event to [axonhub-plugins](https://github.com/looplj/axonhub-plugins) with that commit SHA. The workflow there checks out the commit, packages new versions, creates tags and GitHub Releases, and updates the market index.

If you maintain your plugin in a separate repository, publish the zip at an HTTPS URL, such as a GitHub Release. Compute `sha256` as 64 lowercase hex characters of the raw zip bytes:

```bash
shasum -a 256 temperature-cap-1.0.0.axplugin
```

To list a plugin maintained separately, open a pull request in [axonhub-plugins](https://github.com/looplj/axonhub-plugins) that adds a plugin object to the `plugins` array in `external-plugins.json`. Merging it to `main` rewrites the index from that file. Each `versions` item needs `version`, `url`, `sha256`, and `axonhubVersion`:

```json
{
  "id": "acme.temperature-cap",
  "name": "Temperature Cap",
  "description": "Caps temperature on chat requests.",
  "versions": [
    {
      "version": "1.0.0",
      "url": "https://github.com/acme/temperature-cap/releases/download/v1.0.0/temperature-cap-1.0.0.axplugin",
      "sha256": "64 lowercase hex characters of the zip",
      "axonhubVersion": "^1.0.0"
    }
  ]
}
```

AxonHub uses the index to verify the downloaded package's SHA-256, plugin ID and version. The owner grants permissions and network access when enabling a plugin.

## Examples

| Example | What it shows |
| --- | --- |
| [Notes](../../../plugins/examples/notes/README.md) | A page, `configSchema`, plugin GraphQL, and this plugin's KV |
| [Event Log](../../../plugins/examples/event-log/README.md) | `onEvent` appending a short record to KV and listing it on a page |
| [Failover Guard](../../../plugins/examples/failover-guard/README.md) | `onEvent` with `ctx.counter`, `ctx.axonhub.channels.disable`, and `ctx.webhook.send` |
| [Star history](../../../plugins/examples/star-history/README.md) | A page calling `axonhub.fetch` against granted `network` rules |
| [Temperature Cap](../../../plugins/examples/temperature-cap/README.md) | `onRequest` returning the request with `temperature` set from config |
| [DeepSeek Harness Session Trace](../../../plugins/examples/dsh-session-trace/README.md) | Server middleware supplying a trace ID from a client session header |

## Limits

Authors run into these limits first:

- The zip is at most 5 MiB.
- `plugin.js` is at most 1 MiB.
- An `onEvent`, `onRequest`, or `onAttempt` call times out after 10 seconds, including an awaited `ctx.fetch`.
- A script error fails open. The previous request is kept and the HTTP status stays unchanged. The event delivery is dropped.
- When an LLM request or attempt hook cannot borrow a VM, the request returns 429 `plugin_overloaded` and is not sent upstream. Server middleware uses the shared stage budget described above.
