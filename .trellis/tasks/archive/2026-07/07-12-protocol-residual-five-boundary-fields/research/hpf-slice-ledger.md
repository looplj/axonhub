# G8 Residual Five Boundary Fields Ledger

| Slice | Outcome | Status |
|---|---|---|
| G8-S3 Chat modalities same-protocol tests | `inbound_test.go` ModalitiesRoundTrip/Omitted | completed |
| G8-S4 Anthropic top-level cache_control tests | `cache_control_test.go` TestTopLevelCacheControlRoundTrip | completed |
| G8-S1 Responses context_management field test | `g8_field_preservation_test.go` | completed |
| G8-S2 Responses conversation field test | same file, string+object | completed |
| G8-S5 Hosted tools inventory + raw/lossy coverage | same file | completed |
| Matrix evidence sync | strict verification matrix rows elevated to PARTIAL with field tests | completed |
| Independent review | `research/reviews/g8-residual-five-fields-review.md` PASS | completed |

## Production code
No production transformer changes. Existing paths already implemented.

## Explicit non-claims
- Not 101-row CONFIRMED
- Raw same-protocol fidelity != cross-protocol semantic equivalence
- No unified hosted cross-protocol abstraction
