# PRD — High-Priority Fixtures and Matrix Sync

## Goal

在所有 implementation modules 完成后，补齐 Round 4/5 中高优先 `Needs fixture/test only` 项，或为不适用项写出基于协议和代码的明确理由；同步主矩阵、batch drafts、Trellis spec 与本地 evidence。

## Required Behavior

1. 不在此任务引入新 protocol feature；需要新 feature 的项回到对应 implementation module。
2. 每个高优先 fixture-only row 都有：fixture/test、或明确 unsupported/not-applicable reason。
3. 主矩阵只记录已验证实现、已验证 lossy/unsupported、或明确 source gap；不得把计划当完成。
4. batch draft、Trellis spec、summary 与代码一致。

## Acceptance Criteria

- 高优先 fixture backlog 全部关闭或有可复核理由。
- 101-row readiness 的状态变化有路径和 test evidence。
- 所有 matrix update 行能追溯到 P0/P1/P2/P3 evidence。
- final parent check 无未处理 matrix/spec inconsistency。

