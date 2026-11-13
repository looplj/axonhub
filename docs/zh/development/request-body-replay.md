# 请求正文存储回放

[English](../../en/development/request-body-replay.md)

## 测量方法

`TestRequestBodyPayloadReplayFile` 将按顺序记录的 request/execution JSON 对送入 `CreateRequest` 和 `CreateRequestExecution`，报告逐轮字节复用率、新增唯一块正文、清单、引用、input 消息变化和持久化耗时。计时包含两次提交及分块、哈希工作，不包含初始化、读取输入文件、指标计算和往返校验；不会调用上游 API。

默认 `TestRequestBodyPayloadReplayAppendOnly` 使用 16 轮确定性的增长正文和临时 SQLite 文件。私有回放需要显式指定：

```sh
go test ./internal/server/biz -run '^TestRequestBodyPayloadReplay' -count=1 -v
AXONHUB_TEST_PAYLOAD_REPLAY_FILE=/absolute/path/agent.jsonl \
AXONHUB_TEST_PAYLOAD_PG_DSN='postgres://postgres@127.0.0.1:5432/axonhub_payload_test?sslmode=disable' \
go test ./internal/server/biz -run '^TestRequestBodyPayloadReplayFile$' -count=3 -v
```

JSONL 每行只有 `request_body` 和 `execution_body`，值均为原始 JSON 对象。按时间顺序为每个请求选择一次已保存的执行，保留原始字节，不要解析后重新序列化采集内容。测试仅输出指标和文件 SHA-256，不输出正文。PostgreSQL 仅允许回环地址上的专用 `axonhub_payload_test` 数据库，并创建、删除独立 schema。未指定 PostgreSQL DSN 时，使用 `synchronous=FULL` 的临时持久化 SQLite 库。

与原实现比较时，将本测试复制到 `e24549302eee4efe25ba06092712ef023a032f24` 的独立 detached checkout，使用相同输入和数据库配置。完整正文对照组仅在该临时 checkout 中关闭 `CreateRequest`、`CreateRequestExecution` 的两处 `PrepareChunkedRequestBody` 赋值，继续走原有内联持久化路径。这只是测量改动，不新增产品开关。各方案顺序运行，每次使用新 schema。

## 真实会话样本

2026-11-14 测量了一条已有 Codex 会话约 72 分钟内的 96 次连续调用，每次取首次持久化执行：共 192 份正文、197,806,089 原始字节（188.64 MiB）。样本除了追加，还包括历史缩减和请求元数据变化。正文从已有存储还原，逐份核验已保存的长度和 SHA-256；私有输入和请求标识不进入仓库。

输入文件 SHA-256：`4f05242866feda023fdb5f3aff42a66b9e4a114e307f4dfb552661dfc1f02ec2`。

环境：Go 1.27.4、ARM64 macOS、本机 PostgreSQL 16 容器、单连接单写入者、无外键，`fsync=on`、`synchronous_commit=on`、`full_page_writes=on`、默认 `pglz` TOAST 压缩。下表取三次运行的中位数，每次延迟分位数覆盖 96 对正文。数据库空间是 request、execution、chunk、reference 四表的 `pg_total_relation_size` 增量，包含索引与 TOAST，不含 WAL。

| 方案 | 唯一块正文 | 引用数 | 数据库空间增量 | 每对 P50 | 每对 P95 |
|---|---:|---:|---:|---:|---:|
| 完整正文对照 | 197,806,089 B 内联 | 0 | 149.45 MiB | 33.27 ms | 120.59 ms |
| 原 PR（`e2454930`） | 14,094,340 B | 8,724 | 13.33 MiB | 25.12 ms | 69.97 ms |
| 写入优化与有限尾部分块 | 10,326,855 B | 9,823 | 10.74 MiB | 24.48 ms | 76.45 ms |

最终方案的逻辑正文字节复用率为 94.78%；加上 23,489 字节清单，逻辑载荷节省为 94.77%。实测数据库空间比完整正文少 92.81%，比原 PR 少 19.40%，引用数增加 12.60%。最终三次运行 P50 为 23.73–24.69 ms，P95 为 69.17–76.79 ms。本样本 P95 比原 PR 略高，因此不声称所有延迟指标都改善。

## 小量追加与尾部策略

正文两端同时变化时，原始字节的共同前后缀差分会把中间未变的历史也算入跨度。回放分别记录该原始跨度、`input` 消息条目变化和外围字段变化。严格追加要求此前每个 input 条目保持不变，再追加新条目。以下子组排除初始 seed，两条 lane 各有 21 次、每次新增 input 条目不超过 4 KiB：

| 记录类型 | 新增 input 条目 | 原方案新增块正文 | 最终新增块正文 | 原/最终块正文与新增 input 比例 |
|---|---:|---:|---:|---:|
| 请求 | 48,681 B | 449,733 B | 92,431 B | 9.24 倍 / 1.90 倍 |
| 执行 | 47,376 B | 801,237 B | 83,217 B | 16.91 倍 / 1.76 倍 |

该比例包含外围字段变化和粗块边界影响，是新增逻辑载荷，不等于 SQL 传输或 WAL 写放大。消息和字段变化跨度不包含 JSON key、分隔符及已删除字节，也不是最短编辑距离；纯消息追加时，新增条目字节数是精确的。初始请求和执行的 input seed 分别为 3,460,485 B、3,448,019 B，单独统计。

全历史小块实验（最小 2 KiB、12 位 Gear mask）把唯一载荷降到 9,506,097 B，却需要 26,054 条引用；数据库增量 13.27 MiB，P50/P95 为 25.52/78.05 ms。最终方案保留粗内容边界，将原始 `input` 数组内容与外围字段分离，仅把尚未封口的尾部在消息末尾、达到至少 1 KiB 后切开；尾部形成完整粗块时允许合并。这样无需为每条历史消息增加引用。

所有块仍是原始字节切片，最大 64 KiB。清单仍为 version 1，旧分块边界继续可读。无效 JSON、重复 `input` key 和非数组输入回落到原来的整正文 CDC；本次跟进不需要 schema 迁移或历史改写。

## 限制

这只是单条会话在本地单写入者环境的回放，不能代表生产、并发、网络、WAL 或所有供应商的性能。无状态尾部策略仍会保留历史尾片段直到所属记录过期，新内容也可能使更大粗块封口。回放逐字节校验所有分块正文；PostgreSQL 会规范化内联 JSON，因此内联对照按 JSONB 等价性校验。
