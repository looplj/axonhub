# Design — Fixtures and Matrix Sync

## Scope Boundary

本任务只做：

```text
fixture/test coverage
evidence reconciliation
matrix/draft/spec synchronization
```

本任务不做：

```text
new native field support
new cross-protocol bridge
new provider extension schema
```

发现 implementation gap 时创建/回到对应 module task。

## Evidence Rule

任何完成状态必须同时有：

1. protocol source/value evidence；
2. code assignment/return evidence；
3. targeted test 或真实 sample evidence。

