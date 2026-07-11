# Design — Responses Reasoning / Stream

## Classification

| Family | Owner |
|---|---|
| request `reasoning.context` | Responses request native typed/raw sidecar |
| request `reasoning.generate_summary` | `deprecated-compat`, Responses native typed/raw sidecar |
| response reasoning item | Responses response native/raw output item sidecar |
| reasoning stream events | stream fidelity code only |
| future nested variants | same-protocol raw preserve or explicit diagnostic |

## Boundaries

- request reasoning != response reasoning item
- reasoning item != usage reasoning tokens
- stream option != stream event
- OpenAI effort != Anthropic thinking budget

## Micro-Slice Dependencies

```text
8A context -> 8B generate_summary -> 8C output item -> 8D stream -> 8E future variants
```

8D may not start until request/output native storage boundaries are proven, because stream lifecycle metadata must not become a substitute protocol body store.

