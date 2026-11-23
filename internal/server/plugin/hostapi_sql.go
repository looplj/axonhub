// 职责: 让取得 sql 授权的插件执行运行时传入的数据库语句。
// 关联文件:
//
//	internal/server/plugin/hostapi.go
//	internal/server/plugin/invoke.go
//	internal/server/plugin/manifest.go
//	internal/server/db/router.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	使用主库的应用账号并绕过 Ent privacy，每条语句执行前从主库复核授权。

package plugin

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"

	entsql "entgo.io/ent/dialect/sql"

	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/log"
)

const (
	permSQL              = "sql"
	pluginSQLTimeout     = 10 * time.Second
	pluginSQLMaxRows     = 1000
	pluginSQLBytes       = 1 << 20
	pluginSQLMaxArgs     = 256
	pluginSQLConcurrency = 4
	maxSafeJSInteger     = 1<<53 - 1
)

var errPluginSQLResultLimit = errors.New("plugin SQL result exceeds the size limit")

func (h *hostCall) attachSQLAPI(ctx *moejs.Object) error {
	realm := h.rt.Realm()
	api := realm.NewObject()
	if err := defineValue(h.rt, api, "dialect", moejs.String(h.manager.db.Driver().Dialect())); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, api, "query", 2, h.sqlMethod(true)); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, api, "exec", 2, h.sqlMethod(false)); err != nil {
		return err
	}
	return defineValue(h.rt, ctx, "sql", engine.ObjectValue(api))
}

func (h *hostCall) sqlMethod(query bool) moejs.NativeFunc {
	return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if err := h.enter(false); err != nil {
			return moejs.Undefined(), err
		}
		if _, granted := h.granted[permSQL]; !granted {
			return moejs.Undefined(), h.fail(hostPermissionDenied, "sql permission is required")
		}
		statement, err := h.requireString(moejs.Arg(args, 0), "sql")
		if err != nil {
			return moejs.Undefined(), err
		}
		if strings.TrimSpace(statement) == "" || len(statement) > pluginSQLBytes {
			return moejs.Undefined(), h.fail(hostInvalidArgument, "sql must contain between 1 byte and 1 MiB")
		}
		params, err := h.sqlParams(moejs.Arg(args, 1))
		if err != nil {
			return moejs.Undefined(), err
		}
		return h.runSQL(query, statement, params)
	}
}

func (h *hostCall) sqlParams(value moejs.Value) ([]any, error) {
	if value.IsUndefined() {
		return nil, nil
	}
	raw, err := h.rt.AppendJSON(nil, value)
	if err != nil {
		return nil, err
	}
	if len(raw) > pluginSQLBytes {
		return nil, h.fail(hostLimitExceeded, "sql parameters exceed 1 MiB")
	}
	var params []any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err != nil || params == nil || len(params) > pluginSQLMaxArgs {
		return nil, h.fail(hostInvalidArgument, "sql parameters must be an array of at most 256 values")
	}
	for i, param := range params {
		switch value := param.(type) {
		case json.Number:
			number, err := value.Int64()
			if err == nil {
				params[i] = number
				continue
			}
			params[i], err = value.Float64()
			if err != nil {
				return nil, h.fail(hostInvalidArgument, "invalid numeric sql parameter")
			}
		case nil, string, bool:
		default:
			return nil, h.fail(hostInvalidArgument, "sql parameters must be null, string, boolean, or number")
		}
	}
	return params, nil
}

func (h *hostCall) runSQL(query bool, statement string, params []any) (moejs.Value, error) {
	dbDriver := h.manager.db.Driver()
	pool, ok := dbDriver.(interface{ DB() *sql.DB })
	if !ok {
		return moejs.Undefined(), h.fail(hostUnavailable, "sql is unavailable for this database connection")
	}
	ctx, cancel := context.WithTimeout(h.ctx, pluginSQLTimeout)
	defer cancel()
	select {
	case h.manager.sqlSlots <- struct{}{}:
		defer func() { <-h.manager.sqlSlots }()
	case <-ctx.Done():
		return moejs.Undefined(), h.sqlFailure(ctx, ctx.Err())
	}
	conn, err := pool.DB().Conn(ctx)
	if err != nil {
		return moejs.Undefined(), h.sqlFailure(ctx, err)
	}
	defer conn.Close()

	selector := entsql.Dialect(dbDriver.Dialect()).
		Select(entplugin.FieldGrantedPermissions).
		From(entsql.Table(entplugin.Table)).
		Where(entsql.And(
			entsql.EQ(entplugin.FieldID, h.installationID),
			entsql.EQ(entplugin.FieldEnabled, true),
			entsql.EQ(entplugin.FieldDeletedAt, 0),
		))
	grantSQL, grantArgs := selector.Query()
	var grants []byte
	err = conn.QueryRowContext(ctx, grantSQL, grantArgs...).Scan(&grants)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return moejs.Undefined(), h.sqlFailure(ctx, err)
	}
	var current []string
	if err != nil || json.Unmarshal(grants, &current) != nil || !slices.Contains(current, permSQL) {
		return moejs.Undefined(), h.fail(hostPermissionDenied, "sql permission is no longer granted")
	}

	var inMemory bool
	if dbDriver.Dialect() == dialect.SQLite {
		var path string
		if err := conn.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil {
			return moejs.Undefined(), h.sqlFailure(ctx, err)
		}
		inMemory = path == ""
	}
	defer cleanPluginSQLConnection(ctx, conn, inMemory)
	var result []byte
	if query {
		result, err = pluginSQLQuery(ctx, conn, statement, params)
	} else {
		var execResult sql.Result
		execResult, err = conn.ExecContext(ctx, statement, params...)
		if err == nil {
			var affected int64
			affected, err = execResult.RowsAffected()
			if err == nil {
				result, err = json.Marshal(map[string]any{"rowsAffected": sqlJSONValue(affected, "")})
			}
		}
	}

	log.Info(ctx, "plugin sql executed",
		log.String("plugin_id", h.pluginID),
		log.Int("installation_id", h.installationID),
		log.String("export", h.export),
		log.Bool("query", query),
		log.String("statement_sha256", sha256Hex([]byte(statement))),
		log.Bool("success", err == nil))
	if err != nil {
		return moejs.Undefined(), h.sqlFailure(ctx, err)
	}
	return h.rt.ParseJSON(result)
}

func cleanPluginSQLConnection(ctx context.Context, conn *sql.Conn, inMemory bool) {
	// Arbitrary SQL can change session settings or leave a transaction open.
	// Retire the connection before it can be used again. SQLite in-memory data
	// may depend on this connection staying open, so only roll back there.
	if !inMemory {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_, _ = conn.ExecContext(cleanupCtx, "ROLLBACK")
}

func (h *hostCall) sqlFailure(ctx context.Context, err error) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return h.fail(hostDeadlineExceeded, "sql execution timed out")
	case errors.Is(ctx.Err(), context.Canceled):
		return h.fail(hostCanceled, "sql execution canceled")
	case errors.Is(err, errPluginSQLResultLimit):
		return h.fail(hostLimitExceeded, err.Error())
	default:
		return h.fail(hostInternal, "database statement failed")
	}
}

func pluginSQLQuery(ctx context.Context, conn *sql.Conn, statement string, params []any) ([]byte, error) {
	rows, err := conn.QueryContext(ctx, statement, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	columnsJSON, err := json.Marshal(columns)
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer
	result.WriteString(`{"columns":`)
	result.Write(columnsJSON)
	result.WriteString(`,"rows":[`)
	values := make([]any, len(columns))
	destinations := make([]any, len(columns))
	for i := range values {
		destinations[i] = &values[i]
	}
	count := 0
	for rows.Next() {
		if count >= pluginSQLMaxRows {
			return nil, errPluginSQLResultLimit
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		for i, value := range values {
			switch value := value.(type) {
			case string:
				if len(value) >= pluginSQLBytes {
					return nil, errPluginSQLResultLimit
				}
			case []byte:
				if len(value) >= pluginSQLBytes {
					return nil, errPluginSQLResultLimit
				}
			}
			values[i] = sqlJSONValue(value, columnTypes[i].DatabaseTypeName())
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, err
		}
		if result.Len()+len(encoded)+3 > pluginSQLBytes {
			return nil, errPluginSQLResultLimit
		}
		if count > 0 {
			result.WriteByte(',')
		}
		result.Write(encoded)
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.WriteString("]}")
	if result.Len() > pluginSQLBytes {
		return nil, errPluginSQLResultLimit
	}
	return result.Bytes(), nil
}

func sqlJSONValue(value any, columnType string) any {
	switch value := value.(type) {
	case int64:
		if value > maxSafeJSInteger || value < -maxSafeJSInteger {
			return strconv.FormatInt(value, 10)
		}
	case uint64:
		if value > maxSafeJSInteger {
			return strconv.FormatUint(value, 10)
		}
	case float64:
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return strconv.FormatFloat(value, 'g', -1, 64)
		}
	case []byte:
		switch columnType {
		case "CHAR", "VARCHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT", "ENUM", "SET", "JSON", "DECIMAL", "TIME", "DATE", "DATETIME", "TIMESTAMP":
			return string(value)
		}
	}
	return value
}
