# Data Migration (数据迁移)

Data migration provides one-off backfill or repair logic for **upgrading an existing AxonHub instance to a newer version**.

## When it runs

- Data migrations run only when the system has already been initialized (`system_initialized=true`).
- This assumes the database contains data produced by an older version that now needs to be supplemented to meet the new version's requirements.
- For a fresh installation where the system initialization flow has not been executed, data migrations are skipped and no changes are made.

## How it differs from schema migrations

- Schema migrations adjust database structures and run in every scenario.
- Data migrations only run during upgrade scenarios to modify existing data.

## Execution order

1. Run schema migrations to synchronize database schemas.
2. Start the application; once it detects the system is initialized, execute data migrations in version order (the order of `migrator.Register(...)`).
3. After data migrations finish, the system version is updated.

## Recommendations for new data migrations

- Ensure each migration is idempotent so it can safely run multiple times.
- Write unit/integration tests to verify the upgrade path from old data to new data.
- Add necessary logging within the migration to aid troubleshooting.

---

# 数据迁移（Data Migration）

数据迁移用于在 **已有 AxonHub 实例升级到新版本** 时，为历史数据执行一次性的补齐或修复逻辑。

## 何时会执行

- 仅当系统已经初始化完成（`system_initialized=true`）时，数据迁移才会运行。
- 这意味着数据库中已经存在旧版本产生的数据，新版本需要补齐或修复这些数据。
- 如果是全新部署、尚未执行系统初始化流程，则数据迁移会被跳过，不会执行任何操作。

## 和 Schema Migration 的区别

- Schema Migration（结构迁移）负责调整数据库结构，任何场景都会执行。
- Data Migration（数据迁移）仅在升级场景下运行，用于修改现有数据。

## 执行顺序

1. 运行 schema migration，同步数据库表结构。
2. 启动程序并在检测到系统已初始化后，按版本顺序执行数据迁移（`migrator.Register(...)` 的顺序）。
3. 数据迁移执行完成后，会更新系统版本号。

## 新增数据迁移的建议

- 迁移过程需要兼容多次执行（幂等）。
- 编写对应的单元或集成测试，验证从旧数据升级到新数据的流程。
- 在迁移中添加必要的日志，便于排查问题。
