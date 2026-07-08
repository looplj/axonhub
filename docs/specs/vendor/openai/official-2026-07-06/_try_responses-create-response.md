[Skip to content](https://developers.openai.com/api/reference/overview#_top)

API Reference

[Introduction](https://developers.openai.com/api/reference/overview)

# API Overview

Use this reference to look up OpenAI API endpoints, request and response
schemas, streaming events, client library methods, and shared behavior such as
authentication, errors, rate limits, and request IDs.

## Start here

[Section titled “Start here”](https://developers.openai.com/api/reference/overview#start-here)

1. Choose the API surface for your application:
   - [Responses](https://developers.openai.com/api/reference/responses/overview) for direct model requests, tool use, audio, image, and text inputs, and stateful interactions.
   - [Realtime API](https://developers.openai.com/api/docs/guides/realtime) for low-latency voice or audio sessions over WebRTC, WebSocket, or SIP. Use the [client events](https://developers.openai.com/api/reference/resources/realtime/client-events) and [server events](https://developers.openai.com/api/reference/resources/realtime/server-events) references while building sessions.
   - [Administration](https://developers.openai.com/api/reference/administration/overview) for organization workflows such as users, invites, projects, API keys, and audit logs.
2. Create credentials. Use a standard [API key](https://platform.openai.com/settings/organization/api-keys) for application requests, an [Admin API key](https://platform.openai.com/settings/organization/admin-keys) for Administration endpoints, or [workload identity federation](https://developers.openai.com/api/docs/guides/workload-identity-federation) for short-lived access tokens.
3. Install an official client library from the [libraries page](https://developers.openai.com/api/docs/libraries), or call the HTTP API directly from any environment that supports HTTP requests.
4. Make a first request with the [developer quickstart](https://developers.openai.com/api/docs/quickstart) or go straight to the [Responses create reference](https://developers.openai.com/api/reference/resources/responses/methods/create).
5. Before production, review [error codes](https://developers.openai.com/api/docs/guides/error-codes), [rate limits](https://developers.openai.com/api/docs/guides/rate-limits), and request ID logging below.

## Authentication

[Section titled “Authentication”](https://developers.openai.com/api/reference/overview#authentication)

The OpenAI API accepts bearer credentials from API keys or from short-lived access tokens created with [workload identity federation](https://developers.openai.com/api/docs/guides/workload-identity-federation).

**Remember that your API key is a secret.** Don’t share it with others or expose it in any client-side code such as browsers or apps. Load API keys from an environment variable or key management service on the server.

Provide API credentials with [HTTP Bearer authentication](https://swagger.io/docs/specification/v3_0/authentication/bearer-authentication/).

```
Authorization: Bearer OPENAI_API_KEY_OR_ACCESS_TOKEN


```

If you belong to more than one organization or access projects through a legacy user API key, pass a header to specify which organization and project to use for an API request:

```
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "OpenAI-Organization: $ORGANIZATION_ID" \
  -H "OpenAI-Project: $PROJECT_ID"


```

Usage from these API requests counts as usage for the specified organization and project. Find organization and project IDs in your [dashboard settings](https://platform.openai.com/settings/organization/general).

## Debugging requests

[Section titled “Debugging requests”](https://developers.openai.com/api/reference/overview#debugging-requests)

[Error codes](https://developers.openai.com/api/docs/guides/error-codes) describe failures returned from API responses. Inspect HTTP response headers for the unique ID of a request and rate limit details. Common response headers include:

**API meta information**

- `openai-organization`: The [organization](https://developers.openai.com/api/docs/guides/production-best-practices#setting-up-your-organization) associated with the request
- `openai-processing-ms`: Time taken processing your API request
- `openai-version`: REST API version used for this request (currently `2020-10-01`)
- `x-request-id`: Unique identifier for this API request (used in troubleshooting)

**[Rate limiting information](https://developers.openai.com/api/docs/guides/rate-limits)**

- `x-ratelimit-limit-requests`
- `x-ratelimit-limit-tokens`
- `x-ratelimit-remaining-requests`
- `x-ratelimit-remaining-tokens`
- `x-ratelimit-reset-requests`
- `x-ratelimit-reset-tokens`
- `x-ratelimit-limit-project-tokens`
- `x-ratelimit-remaining-project-tokens`
- `x-ratelimit-reset-project-tokens`

Project-token headers may be present when a project-scoped token limit applies.

**OpenAI recommends logging request IDs in production deployments** for more efficient troubleshooting with the [support team](https://help.openai.com/en/), should the need arise. Official [client libraries](https://developers.openai.com/api/docs/libraries) provide a property on top-level response objects containing the value of the `x-request-id` header.

### Supplying your own request ID with `X-Client-Request-Id`

[Section titled “Supplying your own request ID with X-Client-Request-Id”](https://developers.openai.com/api/reference/overview#supplying-your-own-request-id-with-x-client-request-id)

Along with the server-generated `x-request-id`, you can supply your own unique identifier for each request via the `X-Client-Request-Id` request header. This header isn’t added automatically; you must explicitly set it on the request.

When you include `X-Client-Request-Id`:

- You control the ID format (for example, a UUID or your internal trace ID), but it must contain only ASCII characters and be no more than 512 characters long; otherwise, the request will fail with a 400 error. Make this value unique per request.

- OpenAI logs this value internally for supported endpoints, including chat/completions, embeddings, responses, and more.

- In cases like timeouts or network issues when you can’t get the `X-Request-Id` response header, you can share the `X-Client-Request-Id` value with the support team to look up whether OpenAI received the request and when.


**Example:**

```
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "X-Client-Request-Id: 123e4567-e89b-12d3-a456-426614174000"


```

## Backwards compatibility

[Section titled “Backwards compatibility”](https://developers.openai.com/api/reference/overview#backwards-compatibility)

OpenAI provides stability to API users by avoiding breaking changes in major API versions whenever reasonably possible. This includes:

- The REST API (currently `v1`)
- First-party [client libraries](https://developers.openai.com/api/docs/libraries) (released libraries adhere to [semantic versioning](https://semver.org/))
- [Model](https://developers.openai.com/api/docs/models) families (like `gpt-4o` or `o4-mini`)

**Model prompting behavior between snapshots is subject to change**.
Model outputs are by their nature variable, so expect changes in prompting and model behavior between snapshots. The best way to ensure consistent prompting behavior and model output is to use pinned model versions, and to run [evals](https://developers.openai.com/api/docs/guides/evals) for your applications.

**Backwards-compatible API changes**:

- Adding new resources (URLs) to the REST API and client libraries
- Adding new optional API parameters
- Adding new properties to JSON response objects or event data
- Changing the order of properties in a JSON response object
- Changing the length or format of opaque strings, like resource identifiers
- Adding new event types in streaming APIs

See the [changelog](https://developers.openai.com/api/docs/changelog) for a list of backwards-compatible changes and rare breaking changes.
