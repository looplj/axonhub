# Implement — Parent Orchestration

## Execution Sequence

1. 完成 Slice 0 的独立审查、提交和 8091 验证。
2. 逐个激活 child task，严格按 `01 -> 09` 顺序执行。
3. 每个 child task 内先执行其设计中的 micro-slices；不能跳过 red/fixture proof。
4. 每个 child task 通过模块审查后，更新 parent evidence map，再进入下一个 child。
5. 最后执行 `09` 的 fixture/matrix/spec sync，再做 parent-level review。

## Required Per-Slice Evidence

- 对应协议 baseline / batch draft / matrix row。
- 当前缺口的 red test 或输入 fixture。
- target outbound 或 same-protocol replay 的断言。
- targeted test command output。
- `git diff --check`。
- same-protocol / cross-protocol / lossy diagnostic 的明确结论。

## Module Review Dispatch

模块内所有 micro-slices self-check 后，启动与主会话相同模型的独立 reviewers：

1. bug reviewer：边界值、回归、fixture 覆盖。
2. protocol reviewer：基准文档、六方向、P0/P1、lossy boundary。
3. architecture reviewer：owner、sidecar、`llm.Request`、dead code、维护性。

所有 review finding 必须分类为 `fixed`、`accepted with evidence` 或 `open blocker`。存在 open blocker 时模块不得通过。

## Parent Finish Gate

1. 所有 child task 通过。
2. 101-row readiness 中所有 `Needs implementation` 已处理；高优先 fixture-only 已补或有不适用理由。
3. 主矩阵、batch draft、Trellis spec 和实际代码一致。
4. 完成 parent architecture review 与 final diff review。
5. 分组本地提交，不能把无关旧改动混入提交。

