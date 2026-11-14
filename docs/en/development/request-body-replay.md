# Request Body Storage Replay

[中文](../../zh/development/request-body-replay.md)

## Method

`TestRequestBodyPayloadReplayFile` replays ordered request/execution JSON pairs through `CreateRequest` and `CreateRequestExecution`. It reports per-turn byte reuse, new unique chunk payload, manifests, references, changed input items, and committed pair latency. Setup, reading the fixture, metric calculations, and roundtrip verification are outside the timer. Both commits and all chunking/hashing work are inside it. No upstream API is called.

The default `TestRequestBodyPayloadReplayAppendOnly` uses 16 deterministic growing bodies and a temporary SQLite file. The private replay is opt-in:

```sh
go test ./internal/server/biz -run '^TestRequestBodyPayloadReplay' -count=1 -v
AXONHUB_TEST_PAYLOAD_REPLAY_FILE=/absolute/path/agent.jsonl \
AXONHUB_TEST_PAYLOAD_PG_DSN='postgres://postgres@127.0.0.1:5432/axonhub_payload_test?sslmode=disable' \
go test ./internal/server/biz -run '^TestRequestBodyPayloadReplayFile$' -count=3 -v
```

Each JSONL line has exactly `request_body` and `execution_body`, each containing its original JSON object. Use one persisted execution per request in chronological order. Preserve the original bytes rather than parsing and serializing a capture again. The test logs metrics and the file SHA-256, never body text. The PostgreSQL fixture accepts only a loopback `axonhub_payload_test` database and creates and removes its own schema. Without a PostgreSQL DSN it uses a temporary, persistent SQLite database with `synchronous=FULL`.

For a comparison with the previous implementation, copy this test file to a detached checkout of `e24549302eee4efe25ba06092712ef023a032f24` and use the same fixture and database configuration. For the inline control, disable only the two `PrepareChunkedRequestBody` assignments inside `CreateRequest` and `CreateRequestExecution` in that disposable checkout; keep the existing inline persistence path. This is a measurement change, not a product setting. Run variants sequentially, with a fresh schema per run.

## Real session sample

Measured on 2026-11-14 using 96 chronological calls from one existing Codex session over about 72 minutes. Each call includes its first persisted execution: 192 bodies and 197,806,089 original bytes (188.64 MiB). The session includes history reduction and changing request metadata as well as appends. Bodies were reconstructed from existing storage and verified against their saved length and SHA-256. The private input and request identifiers are not checked in.

Fixture SHA-256: `4f05242866feda023fdb5f3aff42a66b9e4a114e307f4dfb552661dfc1f02ec2`.

Environment: Go 1.27.4, ARM64 macOS, local PostgreSQL 16 container, one database connection and one writer, no foreign keys, `fsync=on`, `synchronous_commit=on`, `full_page_writes=on`, default `pglz` TOAST compression. Values below are medians of three runs; latency percentiles are calculated over the 96 pairs in each run. Database space is the increase in `pg_total_relation_size` for requests, executions, chunks, and references, including indexes and TOAST. WAL is excluded.

| Variant | Unique chunk payload | References | Database space increase | Pair P50 | Pair P95 |
|---|---:|---:|---:|---:|---:|
| Inline control | 197,806,089 B inline | 0 | 149.45 MiB | 33.27 ms | 120.59 ms |
| Previous PR (`e2454930`) | 14,094,340 B | 8,724 | 13.33 MiB | 25.12 ms | 69.97 ms |
| Write optimization + bounded tail | 10,326,855 B | 9,823 | 10.74 MiB | 24.48 ms | 76.45 ms |

The final variant reuses 94.78% of logical body bytes; including 23,489 manifest bytes, logical payload savings are 94.77%. Measured database space is 92.81% below the inline control and 19.40% below the previous PR. Reference count rises 12.60%. Across the final three runs P50 ranged from 23.73 to 24.69 ms and P95 from 69.17 to 76.79 ms. P95 is slightly worse than the previous PR in this sample; the change does not claim an across-the-board latency improvement.

## Small appends and tail policy

Changes at both ends of a body can make a raw prefix/suffix diff include unchanged history. The replay therefore reports that raw span separately from changes to individual `input` items and other top-level values. A strict append preserves every previous input item and adds new ones. The following subgroup excludes the first seed and includes 21 strict appends per lane, each adding at most 4 KiB of input item bytes:

| Lane | Added input item bytes | Previous new chunk payload | Final new chunk payload | Previous / final payload-to-input ratio |
|---|---:|---:|---:|---:|
| Request | 48,681 B | 449,733 B | 92,431 B | 9.24x / 1.90x |
| Execution | 47,376 B | 801,237 B | 83,217 B | 16.91x / 1.76x |

These ratios include chunks affected by changing envelope fields and coarse boundaries; they are logical newly stored payload, not SQL traffic or WAL write amplification. Item/field change spans exclude JSON keys, separators, and deleted bytes and are not minimum edit distance. For pure item appends the new item byte count is exact. The initial request and execution input seeds were 3,460,485 B and 3,448,019 B and are reported separately.

A whole-history smaller-block experiment (2 KiB minimum, 12-bit Gear mask) reduced unique payload to 9,506,097 B but required 26,054 references. Its database increase was 13.27 MiB and P50/P95 were 25.52/78.05 ms. Instead, the final policy retains the original coarse content boundaries, separates the raw `input` array contents from its surrounding envelope, and splits only the unfinished tail at item ends after at least 1 KiB. When that tail becomes a completed coarse block it can be coalesced. This avoids assigning a reference to every historical message.

All chunks remain original byte slices with a 64 KiB maximum. The manifest stays at version 1, and previously stored chunk boundaries remain readable. Invalid JSON, duplicate `input` keys, and non-array inputs retain the previous whole-body CDC path. No schema migration or historical rewrite is needed for this follow-up.

## Limits

This is one session on a local database with one writer. It does not establish production, concurrent, network, WAL, or all-provider performance. The stateless tail policy still stores some historical tail fragments until their owners expire, and new content can seal a larger coarse block. The replay checks every reconstructed chunked body byte-for-byte; PostgreSQL inline bodies are checked for JSONB equality because PostgreSQL normalizes their representation.
