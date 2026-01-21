---
alwaysApply: false
globs: frontend/**/*.ts, frontend/**/*.tsx
---
# Frontend Rules

1. DO NOT restart the development server, it's already managed.

2. We use pnpm as the package manager, run `pnpm dev` to start the development server.

3. Use GraphQL input to filter data instead of filtering in the frontend.

4. Update GraphQL query and schema when adding new fields.

5. Search filters should use debounce to avoid excessive requests.

6. Add sidebar data and route when adding new feature pages.

7. Use `extractNumberID` to extract int ID from the GUID.

8. DO NOT RUN LINT AND BUILD COMMANDS.

## i18n Rules

1. MUST add i18n keys in `locales/*.json` files if creating new keys in code.

2. MUST keep keys in code and JSON files identical.

3. The amount must be formatted with a currency symbol.
   e.g
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

## UI Components

1. When using `AutoComplete` or `AutoCompleteSelect` inside a `Dialog`, MUST pass `portalContainer` prop pointing to the Dialog's container element to fix scrolling issues.
