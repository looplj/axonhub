---
trigger: glob
globs: *.ts,*.tsx
---

# Frontend rules

1. DO NOT restart the development server, I have started it already.

2. We use the pnpm as the package manager, can run `pnpm dev` to start the development server

3. Use graphql input to filter the data in stead of filter in the frontend.

4. Update graphql query and schema when add new field.

5. search filter should use debounce to avoid too many requests.

6. Add sidebar data and route if add new feature page

7. Use extractNumberID to extract int id from the GUID.


## i18n rules

1. MUST ADD i18n key in the locales/*.json file if created a new key in the code.

2. MUST KEEP THE KEY IN THE CODE AND JSON FILE THE SAME.

## React

1. use useCallback to wrap the callback function to reduce rerender

## UI Components

1. When using AutoComplete or AutoCompleteSelect inside a Dialog, MUST pass `portalContainer` prop pointing to the Dialog's container element to fix scrolling issues.
