# 数据存储层重构方案：消除过量的 S3 Class A 操作

> 状态：设计稿（待评审）
> 范围：`internal/server/biz/data_storage.go`、`internal/server/gc/gc.go`、`internal/server/video_storage`、`internal/server/backup`、`internal/server/api/request_content.go`
> 目标读者：后端工程师
> 结论先行：采用 **务实混合架构（Pragmatic Hybrid）**——只为有 Class A 计费的后端（S3、GCS）引入原生 `ObjectStore` 接口，`Fs`/`WebDAV` 继续走 afero，`Database` 维持 no-op；分阶段落地，Phase 0 用极小改动先吃掉约 70% 的成本。

---

## 1. 背景与问题

线上观测到 S3 出现大量 **Class A 操作**（`PutObject` / `ListObjectsV2` / multipart 三件套等）。经逐条对抗审查（读真实源码核实，结论全部成立），根因如下。

> Class A 定义（本方案关注项）：`PutObject`、`ListObjects(V2)`、`CopyObject`、`CreateMultipartUpload`、`UploadPart`、`CompleteMultipartUpload` 等。`GetObject` / `HeadObject` 为 Class B；`DeleteObject` 不计费。

### 1.1 根因：用 POSIX 文件系统抽象（afero）套在对象存储上

存储层把所有后端统一抽象成 `afero.Fs`，S3 经 `github.com/looplj/afero-s3` 适配。POSIX 语义（`Create`/`Open`/`Write`/`Close`/`Stat`/`Mkdir`）与对象存储语义（`PutObject`/`GetObject`/`DeleteObject`/`ListObjects`）存在**阻抗失配**，适配器为"模拟文件语义"额外发起了大量 Class A 调用。

### 1.2 已核实的六个放大源

| # | 问题 | 位置 | 后果 |
|---|------|------|------|
| P1 | `SaveData` 一次逻辑写 = **3 个 PutObject** | `data_storage.go:542-553`（`fs.Create`+`f.Close`+`afero.WriteFile`） | `Create` 显式空 PutObject + 该句柄 `Close` 经 upload manager 再刷一个空 PutObject + `WriteFile` 真实 PutObject（应为 1） |
| P2 | `SaveDataFromReader` = **2 个 PutObject** | `data_storage.go:589-595` | `fs.Create` 多余空 PutObject + 真实数据写入 |
| P3 | 每请求 **4~6 次** `SaveData`（请求体/执行请求体/响应体/执行响应体 + chunks），且重试再叠加 | `request.go:246,365,447,632,716,926,1014` | 与 P1 叠乘 → **~12~18 PutObject/请求**（应为 ~4~6） |
| P4 | GC 删除时 `Remove` 先 `Stat`；不存在的键 → `ListObjectsV2` | `gc.go:357,395` → `data_storage.go:628` → afero-s3 `Remove`/`Stat`/`statDirectory` | 3 个"目录标记键"对 S3 从不创建，每次删除**必然**触发 `ListObjectsV2`；未写入的文件键（如关闭 chunks 时）同样触发 |
| P5 | 缺失键的读取升级为 `ListObjectsV2` | `data_storage.go:665`（`LoadData`）、`request_content.go:125`（下载） | 正常读是 Class B（Head+Get），但读不存在的键经 `statDirectory` 变成 Class A |
| P6 | 未配置重试器 + 大对象 multipart | `createS3Fs`（`data_storage.go:388`）；video worker `worker.go:193-198`（512MB `io.LimitReader`，不可 seek → 强制 multipart） | SDK 默认重试 3 次：限流（503 SlowDown）下每个 Class A ×3，形成**限流-重试雪崩**；512MB 视频按默认 5MB 分片 ≈ 100+ 个 `UploadPart` |

### 1.3 明确**不是**问题（不要为其设计）

- `Rename`→`CopyObject`、`Chmod`→`PutObjectAcl`、`RemoveAll`→`ListObjectsV2`：全仓库**无调用方**。
- 桶级操作 `ListBuckets`/`PutBucket*`/`HeadBucket`/`CreateBucket`：**零调用**（`createS3Fs` 仅构造 client）。
- chunks **不是逐块写**——已批量 `json.Marshal` 成一个数组、一次 `SaveData`。
- 每分钟的 `datastorage-fs-reload` cron **不发** S3 请求。

---

## 2. 设计目标与非目标

**目标**

1. 把每次逻辑写从 3（流式 2）降到 **1 个 PutObject**。
2. GC 删除 **0 个 `ListObjectsV2`**：幂等单次 `DeleteObject`，并停止发送从不存在的目录标记键。
3. 缺失键读取 **0 个 Class A**：`GetObject` 404 → `os.ErrNotExist`，无 List 回退。
4. 配置自适应重试，消除限流-重试放大；大对象走可控分片。
5. 五种后端（Database/Fs/S3/GCS/WebDAV）全部继续可用；**已存数据 0 迁移**。

**非目标**

- 不改变对外存储语义、键格式、`StoragePolicy` 门控、DB `{}` 哨兵。
- 不重写无 Class A 计费的后端（Fs 本地、WebDAV 无原生流式 Writer）。
- 不在本方案内合并"请求级 + 执行级"双份落盘（属产品决策，见 §9）。

---

## 3. 现状约束（迁移必须遵守）

来自代码盘点的硬约束：

- **抽象边界**：`DataStorageService` 的公开方法（`SaveData`、`SaveDataFromReader`、`LoadData`、`DeleteData`、`GetFileSystem` 等）被 5 个下游服务依赖，**签名不可变**。
- **键格式不可变**：`/{proj}/requests/{id}/request_body.json` 等（`request.go:73-126`）必须保持字节一致，旧对象升级后仍可读/可删（缓存兼容规则 `.agent/rules/cache-compat.md`）。
- **Database 后端 no-op**：`ds.Primary` / `Database` 类型下 Save/Load/Delete 维持现状（数据在 DB 列里）。
- **三个 afero 逃逸口必须继续工作**（绕过了 `LoadData`/`DeleteData`）：
  - `request_content.go:113,125` 直接 `GetFileSystem` + `fs.Open(key)` 做 HTTP 下载；
  - `autobackup.go:129,134` `afero.ReadDir(fs, "/")` 列备份；
  - `autobackup.go:155` `fs.Remove(name)` 删旧备份。
- **路径规范化**：S3 PathStyle 去前导 `/`、WebDAV 去前导 `/` + 自定义 `mkdirAll`，均需保留。
- **流式**：video worker 经 `SaveDataFromReader` 传最大 512MB 的非 seekable `io.LimitReader`，必须支持流式且不强制小分片。
- **DI**：`NewDataStorageService` 经 FX 注入（`fx_module.go`），新依赖需进 `DataStorageServiceParams`。
- **测试**：现有单测（`data_storage_test.go`、`gc_test.go`、`request_content_test.go`、`backup_test.go`、`restore_test.go`、`request_audio_test.go`）全部基于 `t.TempDir()` 的 FS 后端，必须保持绿。

---

## 4. 目标架构

### 4.1 分层与取舍

```
            DataStorageService  (公开 API 不变，内部按 ds.Type 分发)
            ┌───────────────────────────────────────────────────────┐
            │  SaveData / SaveDataFromReader / LoadData / DeleteData │
            └───────────────┬───────────────────────┬───────────────┘
                            │ S3 / GCS              │ Fs / WebDAV / Database
                            ▼                        ▼
                   ObjectStore（原生）        afero.Fs（保留）        Database：no-op
            ┌──────────────┴──────────────┐   ┌──────┴───────┐
            │ s3ObjectStore  gcsObjectStore│   │ OsFs  WebDAVFs│
            │ aws-sdk-go-v2  cloud storage │   └──────────────┘
            └─────────────────────────────┘
            GetFileSystem(): 对所有非 DB 后端仍返回 afero.Fs（保留逃逸口与 FS 下载快路径）
```

**为什么是混合而不是全量重写**：Class A 的真实成本只在 S3（GCS 次之）；Fs 是本地无计费，WebDAV 的 `gowebdav` 没有原生流式 Writer，重写它们只增加风险与测试负担、零收益。因此把原生代码**精确限定**在 S3+GCS。

### 4.2 `ObjectStore` 接口（新文件 `internal/server/biz/objectstore.go`）

```go
// ObjectStore 是一个最小的对象存储 API，仅对有 Class A 计费的后端（S3、GCS）原生实现。
// Fs/WebDAV 走 afero；Database 在本层之上为 no-op。key 由调用方完成规范化（去前导 '/'）。
type ObjectStore interface {
	// PutObject 单次写入。对内存 []byte 必须使用单次非 multipart 写
	// （S3 PutObject / GCS Writer），绝不 Create+Close+Write。
	PutObject(ctx context.Context, key string, data []byte) error

	// PutObjectStream 从 r 流式写入；size<0 表示长度未知。
	// 仅在确有必要时切到 multipart（S3 manager.Uploader，PartSize 经调优用于 512MB 视频）。
	PutObjectStream(ctx context.Context, key string, r io.Reader, size int64) (written int64, err error)

	// GetObject 读取对象。缺失键必须返回满足 errors.Is(err, os.ErrNotExist) 的错误，且无 List 回退。
	GetObject(ctx context.Context, key string) ([]byte, error)

	// OpenObject 返回流式 reader + size，用于下载路径（request_content）。缺失键 → os.ErrNotExist，无 List 回退。
	OpenObject(ctx context.Context, key string) (body io.ReadCloser, size int64, err error)

	// DeleteObject 删除单键。删除不存在的键（含从未创建的目录标记键、关闭 chunks 时的 chunk 键）
	// 必须是幂等 no-op：单次 DeleteObject，无 HeadObject、无 ListObjectsV2。
	DeleteObject(ctx context.Context, key string) error
}
```

### 4.3 内部分发（`DataStorageService` 公开签名不变）

```go
// objectStoreFor 对 S3/GCS 返回原生 store；否则 (nil,false)，调用方回退到 afero 路径或 no-op。
// store 按 ds.ID 缓存，与 fsCache 同生命周期、同步失效。
func (s *DataStorageService) objectStoreFor(ctx context.Context, ds *ent.DataStorage) (ObjectStore, bool, error) {
	switch ds.Type {
	case datastorage.TypeS3:
		return s.s3StoreFor(ctx, ds)
	case datastorage.TypeGcs:
		return s.gcsStoreFor(ctx, ds)
	default:
		return nil, false, nil // Database -> no-op；Fs/WebDAV -> afero
	}
}

// SaveData:            store,ok := objectStoreFor(); ok → store.PutObject(ctx, normKey, data)；否则 afero（已删除 Create+Close）
// SaveDataFromReader:  ok → store.PutObjectStream(...)；否则 afero OpenFile 路径
// LoadData:            Database → []byte(key)；ok → store.GetObject(...)；否则 afero.ReadFile
// DeleteData:          Database → nil；ok → store.DeleteObject(...)；否则 afero fs.Remove（isNotExist→nil）
// GetFileSystem:       不变，所有非 DB 后端仍 afero（逃逸口 + FS 下载快路径）
```

### 4.4 S3 实现（`internal/server/biz/objectstore_s3.go`）

```go
type s3ObjectStore struct {
	client   *awss3.Client
	uploader *manager.Uploader // PartSize 调优（如 16~32MB），仅供 PutObjectStream 使用
	bucket   string
}

func (o *s3ObjectStore) PutObject(ctx context.Context, key string, data []byte) error {
	_, err := o.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:        &o.bucket,
		Key:           &key,
		Body:          bytes.NewReader(data), // seekable → 单次 PutObject
		ContentLength: aws.Int64(int64(len(data))),
	})
	return err
}

func (o *s3ObjectStore) DeleteObject(ctx context.Context, key string) error {
	_, err := o.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &o.bucket, Key: &key})
	return err // 幂等：缺失键返回 204，绝不 List
}

func (o *s3ObjectStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	out, err := o.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: &o.bucket, Key: &key})
	if err != nil {
		if isS3NotFound(err) { // *types.NoSuchKey / *types.NotFound / smithy 404
			return nil, fmt.Errorf("%w: %s", os.ErrNotExist, key)
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// 客户端构造时安装自适应重试器（消除 503 限流-重试雪崩）：
//   awsconfig.WithRetryer(func() aws.Retryer {
//       return retry.AddWithMaxAttempts(retry.NewAdaptiveMode(), 3)
//   })
```

### 4.5 GCS 实现（复用 `data_storage.go:431` 已创建的 `storage.Client`）

```go
type gcsObjectStore struct{ bucket *storage.BucketHandle }

// PutObject:    w := bucket.Object(key).NewWriter(ctx); w.Write(data); w.Close()  // 单次写
// GetObject:    r, err := bucket.Object(key).NewReader(ctx); ErrObjectNotExist → os.ErrNotExist
// DeleteObject: err := bucket.Object(key).Delete(ctx); ErrObjectNotExist → nil   // 幂等
```

---

## 5. 操作映射（Before / After）

| 操作 | Before（afero-s3） | Before Class A | After（原生 ObjectStore） | After Class A |
|------|--------------------|----------------|---------------------------|---------------|
| `SaveData`（内存 body） | `Create`+`Close`+`WriteFile` | **3 PutObject** + 1 Head(B) | `PutObject(data)` | **1 PutObject** |
| `SaveDataFromReader`（512MB 视频） | `Create`(空) + `io.Copy`（默认 5MB 分片强制 multipart） | 1 空 PutObject + Create/Upload×~100/Complete | `PutObjectStream`（调优 PartSize，<PartSize 单次） | 1 PutObject 或 1 Create + ⌈size/PartSize⌉ Upload + 1 Complete（更少分片、无空 Put） |
| 整条成功请求（4~6 次 body 写，关 chunks） | 4~6 × 3 | **~12~18 PutObject** | 4~6 × 1 | **~4~6 PutObject** |
| 删除目录标记键（从不存在） | `Remove`→`Stat`→NotFound→`statDirectory` | 1 ListObjectsV2（×3 限流重试） | GC 不再发送该键 | **0** |
| 删除真实文件键 | `Remove`→Head + DeleteObject | 1 Head(B) + Delete | `DeleteObject` | **0**（仅 1 Delete） |
| 删除未写入文件键（关 chunks） | `Remove`→Head NotFound→`statDirectory` | 1 ListObjectsV2 | `DeleteObject`（幂等） | **0** |
| GC 清理 N 条请求 | ~5N DeleteData，~2N 升级 List | ~2N ListObjectsV2 | 只发真实键，逐个幂等 Delete | **0 ListObjectsV2** |
| 读缺失键（详情/下载） | `ReadFile`/`Open`→Stat NotFound→`statDirectory` | 1 ListObjectsV2 | `GetObject`→404→`ErrNotExist` | **0**（404 属 Class B） |
| 读存在键 | `ReadFile`（Head+Get） | 0 | `GetObject` | 0（1 GetObject，Class B） |

---

## 6. 次要修复清单（随阶段落地）

| 问题 | 修复 |
|------|------|
| P4 目录标记键 | GC 删除列表移除 3 个目录键（`gc.go:353,390-391`）；原生 `DeleteObject` 对其余缺失键幂等 |
| P5 缺失键读升级 | `GetObject`/`OpenObject` 将 404 映射为 `os.ErrNotExist`，**无 List 回退**；`LoadRequestBody` 等本就吞错返回空 JSON，行为不变 |
| P6 限流重试雪崩 | S3 client 安装 `retry.NewAdaptiveMode()`（客户端限速 + 退避抖动） |
| P6 视频 multipart 碎片 | `PutObjectStream` 用 `manager.Uploader` 显式 PartSize（16~32MB），512MB → ~16~32 分片而非 ~100+ |
| 删除非存在键报错 | 统一 `isNotExist(err)` 助手（覆盖 `os.ErrNotExist` 与 smithy `NotFound`/`NoSuchKey`、GCS `ErrObjectNotExist`） |

---

## 7. 分阶段迁移计划

### Phase 0 —— 极小改动快赢（✅ 已实现；沿用现有 afero 路径，近零风险，吃下约 70% 成本）

> 不引入新接口，仅删除/替换冗余调用。可独立成 PR 先合。

1. **`SaveData`**（`data_storage.go:535-553`）：保留 `isS3PathStyle` 去前导斜杠，**删除** `fs.Create(key)` + `f.Close()`，只保留 `afero.WriteFile`。
   - afero `WriteFile` 自身只产生 1 个 PutObject → SaveData 3→1。
2. **`SaveDataFromReader`**（`data_storage.go:589`）：把 `fs.Create(key)` 换成 `fs.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o777)`。
   - 去掉 `Create` 的冗余空 PutObject → 2→1（大对象仍走 afero-s3 默认 5MB 分片，PartSize 调优留待 Phase 1）。
3. **`createS3Fs`**（`data_storage.go:395-411`）：`LoadDefaultConfig` 增加 `awsconfig.WithRetryer(func() aws.Retryer { return retry.AddWithMaxAttempts(retry.NewAdaptiveMode(), 3) })`。
4. **GC**（`gc.go`）：把 3 个目录标记键（`GenerateExecutionRequestDirKey`、`GenerateRequestExecutionsDirKey`、`GenerateRequestDirKey`）的删除改为**按后端条件执行**——仅对 `Fs`/`WebDAV` 这类有真实目录的后端追加，对象存储（S3/GCS）跳过。
   - 原因：这些目录键在 S3/GCS 上从不作为对象存在，删除只会白费一次 `ListObjectsV2`；但在 FS/WebDAV 上它们是真实目录，GC 仍需清理（现有 `gc_test.go` 用 FS 存储并断言这些目录被删除）。因此**不能无条件删除**，否则会回归 FS/WebDAV 的目录清理并使测试失败。
   - 实现：新增 `hasRealDirectories(t datastorage.Type) bool`（仅 `TypeFs`/`TypeWebdav` 为 true），用它门控目录键的追加。FS 测试无需改键列表；另加 `TestHasRealDirectories` 守护该意图。
   - （可选）当 `policy.StoreChunks==false` 时，GC 跳过 chunk 键，进一步省去未写键的删除往返。

**Phase 0 效果**：每请求 ~12 PutObject + 4 Head → **~4 PutObject**；GC 不再对目录标记键发 `ListObjectsV2`。仍残留：未写文件键删除/缺失键读取的 `ListObjectsV2`（afero `Remove`/`ReadFile` 仍先 `Stat`）——由 Phase 1 收尾。

### Phase 1 —— S3 原生 `ObjectStore`（✅ 已实现）

1. 新增 `internal/server/biz/objectstore.go`（`ObjectStore` 接口 + `objectStoreFor` 分发 + `normalizeObjectKey`）与 `objectstore_s3.go`（`s3ObjectStore` 实现 + `newS3ObjectStore` + `isS3NotFound` + `countingReader`）。
2. `DataStorageService` 在 `SaveData`/`SaveDataFromReader`/`LoadData`/`DeleteData` 顶部加 `objectStoreFor` 分发：S3 走原生 store，其余（Fs/WebDAV/GCS）回退 afero、Database 仍 no-op。
3. 抽出 `newS3Client`（含自适应重试器），由 afero 适配器（`createS3Fs`）与原生 store 共用；原生 client 按 `ds.ID` 缓存进 `objectStoreCache`，挂在 `fsCacheMu` 下、随 `refreshFileSystems`/`Invalidate*` 与 `fsCache` 同步失效；保留 `datastorage-fs-reload` cron 自愈。
4. `GetFileSystem` 不动 → 三个逃逸口与 FS 下载快路径零改动。

实现要点与决策：
- **键兼容**：`normalizeObjectKey` 去前导 `/`，与 afero-s3 `sanitize` 一致（无论 PathStyle），故旧对象（写在 `2/requests/...`）仍可读/写/删。
- **缺失键**：`GetObject`/`OpenObject` 把 `NoSuchKey`/`NotFound`/smithy 404 映射为 `os.ErrNotExist`，**无 List 回退**；`LoadRequestBody` 等本就吞错返回空 JSON，行为不变。
- **删除幂等**：`DeleteObject` 不前置 `Stat`，缺失键返回成功，0 次 `ListObjectsV2`。
- **multipart 受控**：流式上传用 `manager.Uploader` 且 `PartSize=16MiB`（常量 `s3UploadPartSize`），512MB 视频从 ~100 个 `UploadPart` 降到 ~32 个；小对象仍单次 `PutObject`。
- **测试**：新增 `objectstore_s3_test.go` 覆盖 `isS3NotFound`（typed/smithy/negative）与 `normalizeObjectKey`。S3 往返建议后续加 MinIO/testcontainers（当前无在线 S3 harness）。
- **GCS 暂仍走 afero**（留待 Phase 2 原生化）。`OpenObject` 已实现但下载路径（`request_content.go`）本阶段仍用 `GetFileSystem`（afero），与原生写读同键、可正常下载。

**Phase 1 效果**：S3 的写=1、删=幂等单次、缺失读=0 Class A，multipart 受控。

### Phase 2 —— GCS 原生 + 收尾（可选）

1. 新增 `gcsObjectStore`，复用已建 `storage.Client`，纳入 `objectStoreFor`。
2. （可选）下载路径 `request_content.go` 对 S3/GCS 改用 `OpenObject` 流式下载，减少一次额外 `Stat`。
3. （可选）备份清理 `autobackup.go` 的 `ReadDir` 用 `ObjectStore.List`（如扩展接口）替代，避免逐项 Head。

---

## 8. 兼容性与风险

| 维度 | 处理 |
|------|------|
| 键格式 | 全部 `Generate*Key` 不变，**字节一致**；S3 去前导斜杠规范化在分发前完成；旧对象 0 迁移即可读/删 |
| 缓存兼容 | DB `{}` 哨兵、`shouldUseExternalStorage`/`StoragePolicy` 门控不变，满足 `.agent/rules/cache-compat.md` |
| 公开 API | `DataStorageService` 全部签名不变 → 5 个下游服务**零改动编译通过** |
| Database | 维持 no-op（分发命中 `default` 分支） |
| 逃逸口 | `GetFileSystem` 仍 afero，`request_content.go`/`autobackup.go` 不动 |
| 自愈 | 保留每分钟 fs-reload cron 与 `Invalidate*`（不像全量重写那样丢掉自愈，避免配置陈旧风险） |
| DI | 若 GCS store 需新依赖，加入 `DataStorageServiceParams` 并更新 `fx_module.go` |

**主要风险**：

- **缺失键语义变化**：原生 `GetObject` 缺失返回 `os.ErrNotExist`（而非 afero 包装的 `*os.PathError`）。已核查 `LoadRequestBody` 等均吞错返回空值，行为不变；需在 Phase 1 单测覆盖 `errors.Is(err, os.ErrNotExist)`。
- **无 S3 实测环境**：现有单测全为 FS。Phase 1 建议加 MinIO / testcontainers 冒烟测试（见 §9）。
- **S3 兼容存储（MinIO/Ceph）差异**：保留 `PathStyle` 与去斜杠逻辑；`isS3NotFound` 需覆盖 `NoSuchKey`/`NotFound`/404 多种错误形态。

---

## 9. 测试策略

1. **现有 FS 单测保持绿**：`data_storage_test.go`、`gc_test.go`、`request_content_test.go`、`backup_test.go`、`restore_test.go`、`request_audio_test.go`（均 `t.TempDir()`）。Phase 0 仅需更新 `gc_test.go` 的删除键期望。
2. **新增原生 S3 单测（Phase 1）**：用 MinIO/testcontainers 或 aws-sdk stub，断言：
   - `SaveData` → 恰好 1 次 `PutObject`；
   - `DeleteData` 缺失键 → 1 次 `DeleteObject`、0 次 `List`，且不报错；
   - `LoadData` 缺失键 → `errors.Is(os.ErrNotExist)`、0 次 `List`；
   - `PutObjectStream` 小对象单次 PutObject、大对象受控 multipart 分片数。
3. **调用计数断言**：用 mock S3 client 记录方法名序列，回归"每请求 PutObject 数"。

---

## 10. 影响评估（量化）

| 场景 | 现状 | Phase 0 后 | Phase 1/2 后 |
|------|------|-----------|--------------|
| 每请求写（4 次 body，关 chunks） | ~12 PutObject + 4 Head | **~4 PutObject** | ~4 PutObject |
| GC 每请求 | ~3+ ListObjectsV2 | 0（目录键已移除；未写文件键仍残留） | **0 ListObjectsV2** |
| 缺失键读（详情/下载） | 1 ListObjectsV2 | 1 ListObjectsV2（残留） | **0** |
| 512MB 视频 | 1 空 Put + ~100+ UploadPart | ~100+ UploadPart（无空 Put） | **~16~32 UploadPart**（受控 PartSize） |
| 限流场景 | 每 Class A ×3 重试 | 自适应退避，雪崩消除 | 同 |

> 综合：写放大 3×→1×，GC 的 `ListObjectsV2` 归零，缺失读 Class A 归零，限流雪崩消除。Phase 0 即可拿下大头且风险极低。

---

## 11. 验收清单

- [ ] Phase 0：`SaveData` 去 `Create+Close`；`SaveDataFromReader` 改 `OpenFile`；`createS3Fs` 装自适应重试器；GC 去 3 个目录键 + 更新 `gc_test.go`。
- [ ] Phase 1：`ObjectStore` 接口 + S3 实现 + `objectStoreFor` 分发；S3 client 按 `ds.ID` 缓存并随 `Invalidate*` 失效。
- [ ] 缺失键映射 `os.ErrNotExist`，全部 `Load*` 行为不变。
- [ ] 键字节一致；DB/Fs/WebDAV/逃逸口/快路径全部不变。
- [ ] 现有 FS 单测绿；新增原生 S3 计数断言测试。
- [ ] Phase 2（可选）：GCS 原生；下载流式；备份 List。

---

## 12. 进一步可选优化（需产品确认，超出本次架构范围）

- **削减"双份落盘"**：当前请求级与执行级各存一份 body（不同键），是"每请求多次写"的主因之一。若详情 UI 不需要双份，可在单执行场景下省去请求级落盘——但这会改变**存了什么**（非本方案的"怎么存"），且影响 GC/详情页，需单独评估。
- **附录·关键文件**：
  - `internal/server/biz/data_storage.go`（`SaveData` 508-559、`SaveDataFromReader` 564-604、`DeleteData` 608-640、`LoadData` 645-668、`createS3Fs` 387-420、`createGcsFs` 422-446）
  - `internal/server/gc/gc.go`（349-364、386-403）
  - `internal/server/api/request_content.go`（100-139）
  - `internal/server/video_storage/worker.go`（193-198）
  - `internal/server/backup/autobackup.go`（110、129-155）
