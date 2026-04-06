---
alwaysApply: false
globs: "frontend/**/*.ts, frontend/**/*.tsx"
---

# Frontend Rules

## General

1. DO NOT restart the development server — it's already started and managed.
2. We use `pnpm` as the package manager exclusively.
3. DO NOT run lint and build commands unless explicitly asked.
4. Use GraphQL input to filter data instead of filtering in the frontend.
5. Update GraphQL query and schema when adding new fields.
6. Search filters should use debounce to avoid excessive requests.
7. Add sidebar data and route when adding new feature pages.
8. Use `extractNumberID` (from `src/lib/utils.ts`) to extract int ID from the GUID.
9. Project scoping is determined by page semantics, not helper names:
   Project-level pages must explicitly pass project context (`projectId`, `X-Project-ID`, etc.) to data requests that should be scoped to the current project.
   Admin-level pages must not implicitly inherit the currently selected project and should request global data unless the feature is explicitly project-scoped.

## Development Commands

```bash
cd frontend
pnpm install                      # Install dependencies
pnpm dev                          # Start dev server (port 5173)
pnpm format                       # Format code
pnpm knip                         # Check unused dependencies
pnpm test:e2e                     # E2E tests
```

## Development Guides

For detailed development guides, see:
- **Adding a Feature Page**: [docs/en/development/development.md](../../docs/en/development/development.md)
- **Adding a Channel**: [docs/en/development/development.md](../../docs/en/development/development.md)

## i18n Rules

1. MUST add i18n keys in `locales/*.json` files if creating new keys in code.
2. MUST keep keys in code and JSON files identical.
3. Support both English and Chinese translations (`en.json` and `zh.json`).
4. The amount must be formatted with a currency symbol:
   ```ts
   t('currencies.format', {
     val: cost,
     currency: settings?.currencyCode,
     locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
     minimumFractionDigits: 6,
   })
   ```

## React

1. Use `useCallback` to wrap callback functions to reduce re-renders.
2. NO SSR compatibility required - the application is client-side only.

## UI Components

1. When using `AutoComplete` or `AutoCompleteSelect` inside a `Dialog`, MUST pass `portalContainer` prop pointing to the Dialog's container element to fix scrolling issues.
