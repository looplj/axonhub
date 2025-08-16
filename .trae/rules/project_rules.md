The rules for AxonHub

# Rules

1. All SUMMARY FILE SHOULD STORE IN .trae/summary directory if have.

# Backend rules

1. The server in development is managed by air, it will rebuild and start when code changed, so DO NOT restart manually.

2. Use `go build -o axonhub` to build the server to make sure the server is built successfully.

3. Change any ent schema or graphql schema, need to run `make generate` to regenerate models and resolvers.

4. Use `make generate` command to generate GraphQL and Ent code, which will automatically enter the gql directory and run go generate.

5. DO NOT ADD ANY NEW METHOD/STRUCTURE/FUNCTION/VARIABLE IN *.resolvers.go

##  Golang rules

1. USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.


# Frontend rules

1. DO NOT restart the development server, I have started it already.

2. We use the pnpm as the package manager, can run `pnpm dev` to start the development server


## Login

1. Use my@example.com as the email, and pwd123456 as the password to login when need to test the frontend.

## i18n rules

1. ADD i18n key in the i18n file if created a new key in the code.