# Prompt Protection Rules

AxonHub provides a regex-based prompt protection system that allows you to mask or reject sensitive content in API requests before they are sent to AI providers.

## Overview

Prompt Protection Rules enable you to define regex patterns that match sensitive content in user messages. When a match is found, the system can either:

- **Mask** - Replace the matched content with a custom replacement string
- **Reject** - Block the entire request and return an error

This feature is useful for:
- Protecting sensitive data (API keys, passwords, credit card numbers)
- Preventing prompt injection attacks
- Complying with data privacy regulations
- Filtering inappropriate content

## Core Concepts

### Rule Structure

Each Prompt Protection Rule consists of:

| Field | Type | Description |
|-------|------|-------------|
| **Name** | String | Unique identifier for the rule |
| **Description** | String | Optional description of the rule's purpose |
| **Pattern** | String | Regex pattern to match content |
| **Status** | Enum | `enabled`, `disabled`, or `archived` |
| **Settings** | Object | Action configuration and scope settings |

### Settings Configuration

The `settings` object contains:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| **action** | Enum | Yes | `mask` or `reject` |
| **replacement** | String | Required for `mask` | Text to replace matched content |
| **scopes** | Array | Yes | Message roles to apply the rule to |

### Action Types

| Action | Description |
|--------|-------------|
| **mask** | Replaces matched content with the `replacement` string. The request continues with modified content. |
| **reject** | Blocks the request entirely. Returns an error to the client. |

### Scope Types

Scopes define which message roles the rule applies to:

| Scope | Description |
|-------|-------------|
| **system** | System messages (e.g., "You are a helpful assistant") |
| **developer** | Developer messages (used in some API formats) |
| **user** | User messages |
| **assistant** | Assistant messages (previous responses) |
| **tool** | Tool result messages |

You can select multiple scopes. If no scopes are selected, the rule applies to all message types.

## How It Works

### Request Flow

```
┌─────────────────┐
│  Client Request │
│  with messages  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────────────┐
│         Prompt Protection Pipeline              │
│  ┌──────────────────────────────────────────┐  │
│  │ 1. Load all enabled rules (cached)       │  │
│  │ 2. For each rule:                        │  │
│  │    - Check if scope matches message role │  │
│  │    - Apply regex pattern to content      │  │
│  │    - Execute action (mask/reject)        │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│  Masked Request │ OR  │  Error Response │
│  (modified)     │     │  (if rejected)  │
└────────┬────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐
│  AI Provider    │
│  (receives      │
│  protected      │
│  content)       │
└─────────────────┘
```

### Rule Processing Order

Rules are processed in ascending order by ID. When multiple rules match the same content:

1. **Mask actions** are applied sequentially - each rule's replacement is applied to the result of the previous rule
2. **Reject action** immediately terminates processing - the first reject match blocks the request

## Creating Rules

### Via Admin UI

1. Navigate to **Prompt Protection Rules** in the sidebar
2. Click **Create Rule**
3. Configure the rule:
   - Enter a unique **Name**
   - Add an optional **Description**
   - Define the **Pattern** (regex)
   - Select the **Action** (mask or reject)
   - If masking, enter the **Replacement** text
   - Select the **Scopes** to apply the rule to
4. Click **Create**
5. Enable the rule by clicking the status toggle

### Pattern Examples

#### API Keys and Tokens

```
# Match OpenAI API keys
sk-[a-zA-Z0-9]{48}

# Match generic API keys
(?i)api[_-]?key['\"]?\s*[:=]\s*['\"]?[a-zA-Z0-9_-]{20,}

# Match Bearer tokens
Bearer\s+[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+
```

#### Credit Card Numbers

```
# Match Visa/Mastercard numbers
\b(?:\d{4}[-\s]?){3}\d{4}\b

# Match with context
(?i)(credit\s*card|card\s*number)['\"]?\s*[:=]\s*['\"]?\d{13,19}
```

#### Passwords and Secrets

```
# Match password patterns
(?i)password['\"]?\s*[:=]\s*['\"]?[^\s'\"<>]{8,}

# Match AWS secret keys
(?i)aws[_-]?secret[_-]?access[_-]?key['\"]?\s*[:=]\s*['\"]?[a-zA-Z0-9/+=]{40}
```

#### Email Addresses

```
# Match email addresses
\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b
```

#### Phone Numbers

```
# Match US phone numbers
\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b
```

## Usage Examples

### Example 1: Mask API Keys

**Rule Configuration:**
- Name: `mask-api-keys`
- Pattern: `sk-[a-zA-Z0-9]{48}`
- Action: `mask`
- Replacement: `[REDACTED_API_KEY]`
- Scopes: `user`, `system`

**Before:**
```json
{
  "messages": [
    {"role": "user", "content": "My API key is sk-abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ"}
  ]
}
```

**After:**
```json
{
  "messages": [
    {"role": "user", "content": "My API key is [REDACTED_API_KEY]"}
  ]
}
```

### Example 2: Reject Sensitive Data

**Rule Configuration:**
- Name: `reject-credit-cards`
- Pattern: `\b(?:\d{4}[-\s]?){3}\d{4}\b`
- Action: `reject`
- Scopes: `user`

**Request:**
```json
{
  "messages": [
    {"role": "user", "content": "My credit card is 4111-1111-1111-1111"}
  ]
}
```

**Response:**
```json
{
  "error": {
    "message": "Invalid request: request blocked by prompt protection policy",
    "type": "invalid_request_error"
  }
}
```

### Example 3: Scope-Specific Protection

**Rule Configuration:**
- Name: `protect-system-prompt`
- Pattern: `(?i)ignore\s+(previous|all)\s+(instructions|rules)`
- Action: `reject`
- Scopes: `user`

This rule only applies to user messages, protecting against prompt injection attempts while allowing system prompts to contain similar phrases.

## Rule Management

### Status Management

| Status | Description |
|--------|-------------|
| **enabled** | Rule is active and applied to requests |
| **disabled** | Rule is inactive but can be re-enabled |
| **archived** | Rule is soft-deleted and hidden from lists |

### Bulk Operations

You can perform bulk operations on multiple rules:

- **Bulk Enable** - Enable multiple rules at once
- **Bulk Disable** - Disable multiple rules at once
- **Bulk Delete** - Permanently delete multiple rules

### Cache Behavior

Enabled rules are cached for performance:

- Cache refreshes every 30 seconds
- Changes trigger an async cache reload
- Pattern compilation is cached separately

## Best Practices

### Pattern Design

1. **Be Specific** - Avoid overly broad patterns that might match unintended content
2. **Test Patterns** - Use regex testing tools to validate patterns before deploying
3. **Consider Context** - Include context words when appropriate to reduce false positives
4. **Use Anchors** - Use `\b` word boundaries to avoid partial matches

### Rule Organization

1. **Naming Convention** - Use descriptive names that indicate the rule's purpose
2. **Order Matters** - Rules are processed by ID; consider the order for overlapping patterns
3. **Scope Selection** - Only select necessary scopes to minimize processing overhead
4. **Documentation** - Add descriptions explaining the rule's purpose

### Security Considerations

1. **Defense in Depth** - Use multiple rules for critical data types
2. **Regular Review** - Periodically review and update patterns
3. **Monitor Matches** - Check logs for rule matches to identify potential issues
4. **Test Thoroughly** - Verify rules work as expected before enabling in production

## Common Issues

### Q: Why isn't my rule matching?

A: Check the following:
1. Is the rule **enabled**?
2. Does the **scope** match the message role?
3. Is the **regex pattern** correct? Test it with a regex tool
4. Are there any syntax errors in the pattern?

### Q: How do I test a pattern?

A: Use online regex testers like [regex101.com](https://regex101.com) or [regexr.com](https://regexr.com). Select the Go regex flavor for accurate results.

### Q: What happens if multiple rules match?

A:
- **All mask rules** are applied sequentially
- **First reject rule** immediately blocks the request
- Rules are processed in ascending ID order

### Q: Can I use capture groups in replacements?

A: Currently, replacements are static strings. Capture group substitution is not supported. Use the entire matched content replacement approach.

### Q: How do I handle multi-line content?

A: Use the `(?s)` flag for dot-all mode, or use `[\s\S]` to match any character including newlines:

```
(?s)secret:\s*(.*?)(?=---|$)
```

## Related Documentation

- [Channel Management Guide](channel-management.md)
- [API Key Profiles Guide](api-key-profiles.md)
- [Permissions Guide](permissions.md)
- [Tracing Guide](tracing.md)
