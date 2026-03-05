---
description: "Workflow to add a new channel by following docs/en/guides/development.md"
---

# Adding a New AI Provider Channel

1. **Extend the channel enum** in the Ent schema — add the provider key to the `field.Enum("type")` list in `internal/ent/schema/channel.go` and regenerate Ent artifacts.
2. **Wire the outbound transformer** — update the switch in `ChannelService.buildChannel` to construct the correct outbound transformer for the new enum.
3. **Sync the frontend schema** — update:
   - Zod schema in `frontend/src/features/channels/data/schema.ts`
   - Channel configuration in `frontend/src/features/channels/data/constants.ts`
   - Internationalization in `frontend/src/locales/en.json` and `frontend/src/locales/zh.json`
4. Run `make generate` to regenerate code.
5. Follow the full guide: [docs/en/guides/development.md](../../../docs/en/guides/development.md)
