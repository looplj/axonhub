# Design — Chat Output Controls

## Classification

| Field | Classification |
|---|---|
| top-level `audio` | Chat native request object/raw-preserve |
| `prediction` | Chat native request object/raw-preserve |
| `moderation` | Chat native request object/raw-preserve |
| assistant message `audio` | separate response/message path; out of scope |
| Responses image generation moderation | separate hosted-tool semantic; out of scope |

## Seam

Use Chat native request model plus explicit raw top-level replay. For complex typed object + unknown nested fields, merge typed emission with raw nested extension values; do not use all-or-nothing top-level replay.

## Tests

- top-level audio fields and unknown nested option.
- prediction variants.
- moderation object/value.
- regression: assistant message audio is unchanged.
- cross-protocol remains unsupported/lossy, never fake equivalent.

