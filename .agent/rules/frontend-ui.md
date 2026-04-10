---
alwaysApply: false
globs: "frontend/**/*.tsx"
---

# Frontend UI Rules

1. When using `AutoComplete` or `AutoCompleteSelect` inside a `Dialog`, pass `portalContainer` pointing at the dialog container element to avoid scroll and layering issues.
