// 职责: 验证 MySQL 重复读快照下并发正文写入可复用新提交的数据块。
// 关联文件:
//
//	internal/server/biz/request_body_chunks.go
//	internal/ent/schema/request_body_chunk.go
//
// 注意事项: 仅接受显式配置的本机 axonhub_payload_test 数据库；每次使用独立数据库。

package biz

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	dbsql "database/sql"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestbodychunk"
)

func TestChunkedRequestBodyMySQLConcurrentSnapshot(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PAYLOAD_MYSQL_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PAYLOAD_MYSQL_DSN is not configured")
	}
	config, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "tcp", config.Net)
	host, _, err := net.SplitHostPort(config.Addr)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, host)
	require.Equal(t, "axonhub_payload_test", config.DBName)
	bootstrap, err := dbsql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bootstrap.Close() })
	database := fmt.Sprintf("axonhub_payload_test_%d", time.Now().UnixNano())
	_, err = bootstrap.ExecContext(t.Context(), "CREATE DATABASE `"+database+"`")
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := bootstrap.ExecContext(ctx, "DROP DATABASE `"+database+"`")
		require.NoError(t, err)
	})
	config.DBName = database
	config.ParseTime = true
	db, err := dbsql.Open("mysql", config.FormatDSN())
	require.NoError(t, err)
	db.SetMaxOpenConns(4)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(authz.WithTestBypass(t.Context()), 30*time.Second)
	defer cancel()
	require.NoError(t, client.Schema.Create(ctx, migrate.WithForeignKeys(false)))
	project := client.Project.Create().SetName("mysql-chunks").SaveX(ctx)
	body := chunkTestBody()
	plan := PrepareChunkedRequestBody(body)
	writer, err := client.Tx(ctx)
	require.NoError(t, err)
	defer writer.Rollback()
	owner := writer.Client().Request.Create().SetProjectID(project.ID).SetModelID("model").
		SetRequestBody(plan.Manifest).SetRequestBodyChunked(true).SetStatus(request.StatusProcessing).SaveX(ctx)
	require.NoError(t, plan.Save(ctx, writer.Client(), project.ID, owner.ID, 0))

	contender, err := client.Tx(ctx)
	require.NoError(t, err)
	defer contender.Rollback()
	second := contender.Client().Request.Create().SetProjectID(project.ID).SetModelID("model").
		SetRequestBody(plan.Manifest).SetRequestBodyChunked(true).SetStatus(request.StatusProcessing).SaveX(ctx)
	// Establish the reader's snapshot while the first writer is still uncommitted.
	count, err := contender.Client().RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	beforeInsert := make(chan struct{})
	releaseInsert := make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(releaseInsert) })
	driver := &chunkSnapshotTx{Driver: contender.Client().Driver(), transaction: contender.Client().Driver().(dialect.Tx), beforeInsert: beforeInsert, releaseInsert: releaseInsert}
	transactionClient := ent.NewClient(ent.Driver(driver))
	result := make(chan error, 1)
	go func() {
		defer func() {
			if value := recover(); value != nil {
				result <- fmt.Errorf("chunk writer panic: %v", value)
			}
		}()
		result <- plan.Save(ctx, transactionClient, project.ID, second.ID, 0)
	}()
	select {
	case <-beforeInsert:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, writer.Commit())
	release.Do(func() { close(releaseInsert) })
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, contender.Commit())
	loaded, err := LoadChunkedRequestBody(ctx, client, project.ID, second.ID, 0, second.RequestBody)
	require.NoError(t, err)
	require.Equal(t, body, []byte(loaded))
}

type chunkSnapshotTx struct {
	dialect.Driver

	transaction   dialect.Tx
	beforeInsert  chan struct{}
	releaseInsert chan struct{}
	once          sync.Once
}

func (tx *chunkSnapshotTx) Commit() error { return tx.transaction.Commit() }

func (tx *chunkSnapshotTx) Rollback() error { return tx.transaction.Rollback() }

func (tx *chunkSnapshotTx) Exec(ctx context.Context, query string, args, result any) error {
	if strings.HasPrefix(query, "INSERT INTO `"+requestbodychunk.Table+"`") {
		tx.once.Do(func() {
			close(tx.beforeInsert)
			select {
			case <-tx.releaseInsert:
			case <-ctx.Done():
			}
		})
	}
	return tx.transaction.Exec(ctx, query, args, result)
}

func (tx *chunkSnapshotTx) Query(ctx context.Context, query string, args, rows any) error {
	return tx.transaction.Query(ctx, query, args, rows)
}
