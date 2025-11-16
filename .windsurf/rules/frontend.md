---
trigger: glob
globs: *.ts,*.tsx
---

# Frontend rules

1. DO NOT restart the development server, I have started it already.

2. We use the pnpm as the package manager, can run `pnpm dev` to start the development server

3. Use graphql input to filter the data in stead of filter in the frontend.

4. search filter should use debounce to avoid too many requests.

5. Add sidebar data and route if add new feature page

6. Use extractNumberID to extract int id from the GUID.


## Login

1. Use my@example.com as the email, and pwd123456 as the password to login when need to test the frontend.


## i18n rules

1. MUST ADD i18n key in the locales/*.json file if created a new key in the code.

2. MUST KEEP THE KEY IN THE CODE AND JSON FILE THE SAME.

## React

1. use useCallback to wrap the callback function to reduce rerender