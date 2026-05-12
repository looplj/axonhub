---
description: "Workflow to add a new AI provider channel"
---

# Adding a New AI Provider Channel

## 1. Backend: Ent Schema & Code Generation

1. **Extend the channel type enum** — add the provider key to `field.Enum("type")` in `internal/ent/schema/channel.go:37-92`.
2. **Regenerate** — run `make generate` to regenerate Ent artifacts and GraphQL types after the schema change.
3. **Map default endpoints** — add an entry to `defaultEndpointsForChannelType` in `internal/server/biz/channel_endpoint.go:90-156`.
   - If the channel uses a new API format, also add it to `SupportedAPIFormats` in the same file.
4. **Wire the outbound transformer** — add a `case` to the switch in `ChannelService.buildChannelWithTransformer` (`internal/server/biz/channel_llm.go:348-951`).
   - If the channel uses a credential variant not covered by the `default` case, add credential validation above the switch (lines 320-343).
   - If adding a new transformer package under `llm/transformer/`, import it and build the config with `getAPIKeyProvider(ch)`.

## 2. Frontend: Schema, Config & i18n

1. **Zod schema** — add the new type value to `channelTypeSchema` enum in `frontend/src/features/channels/data/schema.ts:48-102`.
   - If a new API format, add to `apiFormatSchema` and `configurableChannelEndpointApiFormats` in the same file.
2. **Channel config** — add a `CHANNEL_CONFIGS` entry in `frontend/src/features/channels/data/config_channels.ts` (icon, baseURL, defaultModels, apiFormat, color).
3. **Provider config** — add a `PROVIDER_CONFIGS` entry in `frontend/src/features/channels/data/config_providers.ts` if a new provider group, or add the new channel type to an existing provider's `channelTypes` array.
4. **Internationalization** — add/update relevant keys in `frontend/src/locales/en.json` and `frontend/src/locales/zh.json`.

## 3. Finalize

- Run `make generate` again to pick up any generated code changes that may have been introduced by the new transformer or endpoint wiring.
- Verify the build compiles (`make build-backend`).
