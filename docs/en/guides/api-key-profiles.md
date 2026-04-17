# API Key Profile Guide

This guide explains how to configure API Key Profiles for model mapping, access control, and profile switching.

## What is an API Key Profile?

An **API Key Profile** lets you:
- **Map models**: Rewrite the model name from the client request
- **Restrict channels**: Limit the API Key to specific channels
- **Restrict models**: Limit the API Key to specific models
- **Switch profiles**: Create multiple profiles for one API Key and choose which one is active

In simple terms, an API Key Profile decides at the **request entry point** what model the request should be treated as first.

## Where API Key Profile Fits in the Request Flow

API Key Profile model mapping is the **first** step in a three-layer pipeline. For the full picture, see [Request Processing Guide](../getting-started/request-processing.md#core-concept-three-layers-of-model-settings).

In short: **API Key Profile renames → Model Association selects channel → Channel renames → Send upstream**

## Common Use Cases

### Use Case 1: Client tools with fixed model names

Many AI tools use fixed model names internally. If you want them to use other models, use API Key Profile model mapping.

```json
{
  "modelMappings": [
    {"from": "claude-sonnet-4-5", "to": "anthropic/claude-3.5-sonnet"}
  ]
}
```

### Use Case 2: Unify model names across clients

```json
{
  "modelMappings": [
    {"from": "gpt4", "to": "gpt-4o"},
    {"from": "gpt-4-turbo", "to": "gpt-4o"}
  ]
}
```

### Use Case 3: Restrict a profile to part of the system

```json
{
  "channelTags": ["production"],
  "modelIDs": ["gpt-4o", "claude-3-sonnet"]
}
```

## Configuration Steps

### Step 1: Open the profile UI

1. Log in to the AxonHub management interface
2. Go to **API Keys**
3. Find the API Key you want to configure
4. Open the **Actions** menu
5. Select **Profiles** or **Configure**

### Step 2: Create a profile

1. Click **Add Profile**
2. Enter a profile name
3. Configure model mappings, channel restrictions, or model restrictions

### Step 3: Configure model mappings

Each mapping has:
- **From**: the model name in the client request
- **To**: the model name that AxonHub should use

Supported matching methods:

#### Exact match

```json
{"from": "gpt-4", "to": "claude-3-opus"}
```

#### Regex match

```json
{"from": "gpt-.*", "to": "claude-3-sonnet"}
```

### Step 4: Set the active profile

1. Select a profile in **Active Profile**
2. Click **Save**
3. The change takes effect immediately

## Rule Matching Order

Model mappings are evaluated in order. **The first matching rule is applied.**

Put more specific rules first and more general rules later.

## FAQ

### Q: Why is model mapping not working?

Check:
1. Is the correct active profile selected?
2. Does the model name match?
3. Is the regex pattern correct?

### Q: What are the requirements for profile names?

- Must be unique within one API Key
- Cannot be empty
- Should be meaningful

### Q: How many profiles should I create?

There is no hard limit, but keep the set small and easy to manage.

## Best Practices

1. **Use descriptive names** like `production` or `openrouter-mapping`
2. **Put specific rules first**
3. **Test before enabling**
4. **Prefer channel tags** over hardcoded channel IDs

## Related Documentation

- [Model Management Guide](model-management.md) - Configure model association
- [Channel Management Guide](channel-management.md) - Configure upstream channels
- [Load Balancing Guide](load-balance.md) - Understand channel selection and failover
- [Request Processing Guide](../getting-started/request-processing.md) - See the full request flow
