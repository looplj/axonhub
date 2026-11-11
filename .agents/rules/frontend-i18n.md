---
alwaysApply: false
globs: "frontend/src/**/*.ts, frontend/src/**/*.tsx, frontend/src/locales/*.json"
---

# Frontend I18n Rules

1. Any new i18n key used in code must be added to both `frontend/src/locales/en.json` and `frontend/src/locales/zh.json`.
2. Keep translation keys identical between code and locale files.
3. Currency amounts must use the existing i18n formatting pattern:

```ts
t('currencies.format', {
  val: cost,
  currency: settings?.currencyCode,
  locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
  minimumFractionDigits: 6,
})
```
