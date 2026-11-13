// 职责: 分块去重、保存并完整还原数据库中的请求正文。
// 关联文件:
//
//	internal/ent/schema/request_body_chunk.go
//	internal/ent/schema/request_body_chunk_ref.go
//	internal/server/biz/request.go
//	internal/server/gc/gc.go
//
// 注意事项: 写块、引用和所有者必须在同一事务；只在同一项目内复用内容。

package biz

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestbodychunk"
	"github.com/looplj/axonhub/internal/ent/requestbodychunkref"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

const (
	requestBodyChunkThreshold = 32 * 1024
	requestBodyChunkMin       = 4 * 1024
	requestBodyChunkMax       = 64 * 1024
	requestBodyChunkMask      = 1<<14 - 1
	requestBodyChunkBatchSize = 100
)

type requestBodyManifest struct {
	Version int    `json:"version"`
	Size    int    `json:"size"`
	SHA256  string `json:"sha256"`
	Chunks  int    `json:"chunks"`
}

// ChunkedRequestBody is a prepared, immutable body and its compact manifest.
type ChunkedRequestBody struct {
	Manifest objects.JSONRawMessage
	chunks   [][]byte
}

// PrepareChunkedRequestBody leaves small values inline and splits large bodies
// by their content so insertions do not shift every subsequent chunk boundary.
func PrepareChunkedRequestBody(body []byte) *ChunkedRequestBody {
	if len(body) < requestBodyChunkThreshold {
		return nil
	}

	body = bytes.Clone(body)
	var chunks [][]byte
	start := 0
	var fingerprint uint64
	for i, b := range body {
		fingerprint = fingerprint<<1 + requestBodyGear[b]
		size := i + 1 - start
		if size >= requestBodyChunkMax || size >= requestBodyChunkMin && fingerprint&requestBodyChunkMask == 0 {
			chunks = append(chunks, body[start:i+1])
			start = i + 1
			fingerprint = 0
		}
	}
	if start < len(body) {
		chunks = append(chunks, body[start:])
	}
	manifest, _ := json.Marshal(requestBodyManifest{Version: 1, Size: len(body), SHA256: requestBodyHash(body), Chunks: len(chunks)})
	return &ChunkedRequestBody{Manifest: manifest, chunks: chunks}
}

// Fixed SplitMix64 values make the Gear fingerprint deterministic across builds.
var requestBodyGear = func() [256]uint64 {
	var gear [256]uint64
	var state uint64
	for i := range gear {
		state += 0x9e3779b97f4a7c15
		value := state
		value = (value ^ value>>30) * 0xbf58476d1ce4e5b9
		value = (value ^ value>>27) * 0x94d049bb133111eb
		gear[i] = value ^ value>>31
	}
	return gear
}()

func requestBodyHash(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func validRequestBodyOwner(projectID, requestID, executionID int) bool {
	return projectID > 0 && (requestID > 0 && executionID == 0 || executionID > 0 && requestID == 0)
}

// Save attaches a prepared body to an owner already created by the caller's transaction.
func (p *ChunkedRequestBody) Save(ctx context.Context, client *ent.Client, projectID, requestID, executionID int) error {
	if p == nil || len(p.chunks) == 0 || !validRequestBodyOwner(projectID, requestID, executionID) {
		return errors.New("invalid chunked request body owner or payload")
	}
	if _, ok := client.Driver().(dialect.Tx); !ok {
		return errors.New("chunked request body must be saved in its owner's transaction")
	}

	_, err := authz.RunAsHost(ctx, client, "save request body chunks", func(hostCtx context.Context) (struct{}, error) {
		var exists bool
		var err error
		if requestID > 0 {
			exists, err = client.Request.Query().Where(request.IDEQ(requestID), request.ProjectIDEQ(projectID)).Exist(hostCtx)
		} else {
			exists, err = client.RequestExecution.Query().Where(requestexecution.IDEQ(executionID), requestexecution.ProjectIDEQ(projectID)).Exist(hostCtx)
		}
		if err != nil {
			return struct{}{}, fmt.Errorf("query request body owner: %w", err)
		}
		if !exists {
			return struct{}{}, errors.New("request body owner not found in project")
		}

		byHash := make(map[string][]byte, len(p.chunks))
		hashes := make([]string, len(p.chunks))
		for i, chunk := range p.chunks {
			hashes[i] = requestBodyHash(chunk)
			byHash[hashes[i]] = chunk
		}
		uniqueHashes := make([]string, 0, len(byHash))
		for hash := range byHash {
			uniqueHashes = append(uniqueHashes, hash)
		}
		// All writers acquire overlapping chunk locks in the same order.
		slices.Sort(uniqueHashes)
		chunkIDs := make(map[string]int, len(byHash))
		for batch := 0; batch < len(uniqueHashes); batch += requestBodyChunkBatchSize {
			keys := uniqueHashes[batch:min(batch+requestBodyChunkBatchSize, len(uniqueHashes))]
			creates := make([]*ent.RequestBodyChunkCreate, len(keys))
			for i, hash := range keys {
				creates[i] = client.RequestBodyChunk.Create().SetProjectID(projectID).SetHash(hash).SetData(byHash[hash])
			}
			// Touch only the hash on conflicts. This holds the row lock against GC
			// until references commit without rewriting the existing large data.
			err := client.RequestBodyChunk.CreateBulk(creates...).OnConflictColumns(requestbodychunk.FieldProjectID, requestbodychunk.FieldHash).
				Update(func(set *ent.RequestBodyChunkUpsert) { set.SetIgnore(requestbodychunk.FieldHash) }).Exec(hostCtx)
			if err != nil {
				return struct{}{}, fmt.Errorf("save request body chunks: %w", err)
			}
			query := client.RequestBodyChunk.Query().Where(requestbodychunk.ProjectIDEQ(projectID), requestbodychunk.HashIn(keys...))
			if client.Driver().Dialect() == dialect.MySQL {
				// Read chunks committed after this transaction's repeatable-read snapshot.
				query.Modify(func(selector *sql.Selector) { selector.ForUpdate() })
			}
			stored, err := query.All(hostCtx)
			if err != nil {
				return struct{}{}, fmt.Errorf("load saved request body chunks: %w", err)
			}
			for _, chunk := range stored {
				if !bytes.Equal(chunk.Data, byHash[chunk.Hash]) {
					return struct{}{}, errors.New("stored request body chunk does not match its content hash")
				}
				chunkIDs[chunk.Hash] = chunk.ID
			}
		}
		for batch := 0; batch < len(hashes); batch += requestBodyChunkBatchSize {
			end := min(batch+requestBodyChunkBatchSize, len(hashes))
			refs := make([]*ent.RequestBodyChunkRefCreate, end-batch)
			for position := batch; position < end; position++ {
				chunkID, ok := chunkIDs[hashes[position]]
				if !ok {
					return struct{}{}, errors.New("saved request body chunk is missing")
				}
				ref := client.RequestBodyChunkRef.Create().SetChunkID(chunkID).SetPosition(position)
				if requestID > 0 {
					ref.SetRequestID(requestID)
				} else {
					ref.SetExecutionID(executionID)
				}
				refs[position-batch] = ref
			}
			if err := client.RequestBodyChunkRef.CreateBulk(refs...).Exec(hostCtx); err != nil {
				return struct{}{}, fmt.Errorf("save request body chunk references: %w", err)
			}
		}
		return struct{}{}, nil
	})
	return err
}

// LoadChunkedRequestBody reconstructs the exact stored bytes or reports corruption.
func LoadChunkedRequestBody(ctx context.Context, client *ent.Client, projectID, requestID, executionID int, manifest []byte) (objects.JSONRawMessage, error) {
	if !validRequestBodyOwner(projectID, requestID, executionID) {
		return nil, errors.New("invalid chunked request body owner")
	}
	var metadata requestBodyManifest
	if err := json.Unmarshal(manifest, &metadata); err != nil {
		return nil, fmt.Errorf("decode request body manifest: %w", err)
	}
	if metadata.Version != 1 || metadata.Size < requestBodyChunkThreshold || metadata.Chunks < 1 || len(metadata.SHA256) != sha256.Size*2 {
		return nil, errors.New("invalid request body manifest")
	}

	return authz.RunAsHost(ctx, client, "load request body chunks", func(hostCtx context.Context) (objects.JSONRawMessage, error) {
		query := client.RequestBodyChunkRef.Query()
		if requestID > 0 {
			query.Where(requestbodychunkref.RequestIDEQ(requestID), requestbodychunkref.ExecutionIDIsNil())
		} else {
			query.Where(requestbodychunkref.ExecutionIDEQ(executionID), requestbodychunkref.RequestIDIsNil())
		}
		refs, err := query.Order(ent.Asc(requestbodychunkref.FieldPosition)).
			WithChunk(func(query *ent.RequestBodyChunkQuery) { query.Where(requestbodychunk.ProjectIDEQ(projectID)) }).All(hostCtx)
		if err != nil {
			return nil, fmt.Errorf("load request body chunk references: %w", err)
		}
		if len(refs) != metadata.Chunks {
			return nil, errors.New("request body chunk count mismatch")
		}
		var body []byte
		for i, ref := range refs {
			chunk := ref.Edges.Chunk
			if ref.Position != i || chunk == nil || len(chunk.Data) == 0 || len(chunk.Data) > requestBodyChunkMax || requestBodyHash(chunk.Data) != chunk.Hash {
				return nil, errors.New("request body chunk is missing or corrupt")
			}
			body = append(body, chunk.Data...)
			if len(body) > metadata.Size {
				return nil, errors.New("request body exceeds manifest size")
			}
		}
		if len(body) != metadata.Size || requestBodyHash(body) != metadata.SHA256 {
			return nil, errors.New("request body checksum mismatch")
		}
		return objects.JSONRawMessage(body), nil
	})
}

// DeleteUnreferencedRequestBodyChunks reclaims one bounded batch without relying
// on foreign keys, which production migrations intentionally do not create.
func DeleteUnreferencedRequestBodyChunks(ctx context.Context, client *ent.Client, batchSize int) (int, error) {
	if batchSize <= 0 {
		return 0, errors.New("request body chunk cleanup batch size must be positive")
	}
	if _, ok := client.Driver().(dialect.Tx); ok {
		return 0, errors.New("request body chunk cleanup needs independent transactions")
	}
	return authz.RunAsHost(ctx, client, "delete unreferenced request body chunks", func(hostCtx context.Context) (int, error) {
		// Parent deletion cannot cascade on installations without foreign keys.
		for {
			orphanIDs, err := client.RequestBodyChunkRef.Query().Where(
				requestbodychunkref.Not(requestbodychunkref.HasRequestWith(request.IDGT(0))),
				requestbodychunkref.Not(requestbodychunkref.HasExecutionWith(requestexecution.IDGT(0))),
			).Limit(batchSize).IDs(hostCtx)
			if err != nil {
				return 0, fmt.Errorf("find orphaned request body references: %w", err)
			}
			if len(orphanIDs) == 0 {
				break
			}
			removed, err := client.RequestBodyChunkRef.Delete().Where(
				requestbodychunkref.IDIn(orphanIDs...),
				requestbodychunkref.Not(requestbodychunkref.HasRequestWith(request.IDGT(0))),
				requestbodychunkref.Not(requestbodychunkref.HasExecutionWith(requestexecution.IDGT(0))),
			).Exec(hostCtx)
			if err != nil {
				return 0, fmt.Errorf("delete orphaned request body references: %w", err)
			}
			if removed == 0 {
				break
			}
		}
		ids, err := client.RequestBodyChunk.Query().Where(requestbodychunk.Not(requestbodychunk.HasReferences())).
			Order(ent.Asc(requestbodychunk.FieldID)).Limit(batchSize).IDs(hostCtx)
		if err != nil {
			return 0, fmt.Errorf("find unreferenced request body chunks: %w", err)
		}
		deleted := 0
		service := AbstractService{db: client}
		for _, id := range ids {
			err := service.RunInTransaction(hostCtx, func(txCtx context.Context) error {
				tx := ent.FromContext(txCtx)
				// Lock one block at a time, before taking the reference snapshot.
				// Writers hold this same lock until their references commit.
				_, err := tx.RequestBodyChunk.Update().Where(requestbodychunk.IDEQ(id)).Modify(func(update *sql.UpdateBuilder) {
					update.Set(requestbodychunk.FieldHash, sql.ExprFunc(func(builder *sql.Builder) { builder.Ident(requestbodychunk.FieldHash) }))
				}).Save(txCtx)
				if err != nil {
					return err
				}
				refs := tx.RequestBodyChunkRef.Query().Where(requestbodychunkref.ChunkIDEQ(id)).Limit(1)
				if tx.Driver().Dialect() == dialect.MySQL {
					// MySQL repeatable-read snapshots may predate the lock wait.
					refs.Modify(func(selector *sql.Selector) { selector.ForUpdate() })
				}
				refIDs, err := refs.IDs(txCtx)
				if err != nil || len(refIDs) > 0 {
					return err
				}
				count, err := tx.RequestBodyChunk.Delete().Where(requestbodychunk.IDEQ(id)).Exec(txCtx)
				if err == nil {
					deleted += count
				}
				return err
			})
			if err != nil {
				return deleted, fmt.Errorf("reclaim request body chunk: %w", err)
			}
		}
		return deleted, nil
	})
}
