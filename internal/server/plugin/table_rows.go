// 职责: 执行托管表的参数化增删改查和游标分页。
// 关联文件:
//
//	internal/server/plugin/hostapi_tables.go
//	internal/server/plugin/table_manifest.go
//	internal/server/plugin/table_ddl.go
//
// 注意事项:
//
//	upsert 使用同一事务和安装锁，通过声明的唯一键精确决定更新目标。
package plugin

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"modernc.org/sqlite"

	"github.com/looplj/axonhub/internal/objects"
)

const maxPluginTableRowBytes = 64 << 10

type pluginTableRows struct {
	tx      *sql.Tx
	dialect string
	name    string
	spec    objects.PluginTableSpec
}

type tableSelectInput struct {
	Where   []tableFilter `json:"where"`
	OrderBy []tableOrder  `json:"orderBy"`
	First   int           `json:"first"`
	After   string        `json:"after"`
}

type tableFilter struct {
	Column string          `json:"column"`
	Op     string          `json:"op"`
	Value  json.RawMessage `json:"value"`
}

type tableOrder struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

type tableCursor struct {
	Query  string            `json:"query"`
	Values []json.RawMessage `json:"values"`
}

func (t *pluginTableRows) run(ctx context.Context, method string, input [][]byte) (any, error) {
	arg := func(i int) []byte {
		if i >= len(input) {
			return nil
		}
		return input[i]
	}
	switch method {
	case "list":
		var options tableSelectInput
		if len(arg(0)) > 0 {
			if err := decodeTableArgument(arg(0), &options); err != nil {
				return nil, err
			}
		}
		return t.list(ctx, options)
	case "insert":
		row, err := t.decodeRow(arg(0), true)
		if err != nil {
			return nil, err
		}
		return t.insert(ctx, row)
	case "upsert":
		var input struct {
			Conflict []string        `json:"conflict"`
			Row      json.RawMessage `json:"row"`
		}
		if err := decodeTableArgument(arg(0), &input); err != nil {
			return nil, err
		}
		row, err := t.decodeRow(input.Row, true)
		if err != nil {
			return nil, err
		}
		return t.upsert(ctx, input.Conflict, row)
	}
	value, err := t.bindValue("id", arg(0))
	if err != nil {
		return nil, err
	}
	id, ok := value.(int64)
	if !ok || id <= 0 {
		return nil, tableArgumentError("id must be positive")
	}
	switch method {
	case "get":
		return t.get(ctx, id)
	case "update":
		row, err := t.decodeRow(arg(1), false)
		if err != nil {
			return nil, err
		}
		return t.update(ctx, id, row)
	case "delete":
		query, args := entsql.Dialect(t.dialect).Delete(t.name).Where(entsql.EQ("id", id)).Query()
		result, err := t.tx.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		count, err := result.RowsAffected()
		return count > 0, err
	default:
		return nil, tableArgumentError("unknown table operation")
	}
}

func (t *pluginTableRows) column(name string) (objects.PluginTableColumn, bool) {
	if name == "id" {
		return objects.PluginTableColumn{Type: "integer"}, true
	}
	spec, ok := t.spec.Columns[name]
	return spec, ok
}

func (t *pluginTableRows) bindValue(column string, raw []byte) (any, error) {
	spec, ok := t.column(column)
	if !ok {
		return nil, tableArgumentError("unknown column " + column)
	}
	if string(raw) == "null" && spec.Nullable {
		return nil, nil
	}
	var value any
	if err := decodeTableArgument(raw, &value); err != nil {
		return nil, err
	}
	switch spec.Type {
	case "string", "text":
		if value, ok := value.(string); ok && utf8.ValidString(value) && !strings.ContainsRune(value, 0) &&
			(spec.Type == "text" || utf8.RuneCountInString(value) <= spec.MaxLength) {
			return value, nil
		}
	case "integer":
		switch value := value.(type) {
		case json.Number:
			if n, err := value.Int64(); err == nil && n >= -maxSafeJSInteger && n <= maxSafeJSInteger {
				return n, nil
			}
		case string:
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				return n, nil
			}
		}
	case "number":
		if value, ok := value.(json.Number); ok {
			if n, err := value.Float64(); err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) {
				return n, nil
			}
		}
	case "boolean":
		if value, ok := value.(bool); ok {
			return value, nil
		}
	}
	return nil, tableArgumentError("invalid value for column " + column)
}

func (t *pluginTableRows) decodeRow(raw []byte, complete bool) (map[string]any, error) {
	var input map[string]json.RawMessage
	if err := decodeTableArgument(raw, &input); err != nil {
		return nil, err
	}
	row := make(map[string]any, len(input))
	for column, raw := range input {
		if column == "id" {
			return nil, tableArgumentError("id is assigned by AxonHub")
		}
		value, err := t.bindValue(column, raw)
		if err != nil {
			return nil, err
		}
		row[column] = value
	}
	if complete {
		for column, spec := range t.spec.Columns {
			if _, exists := row[column]; !exists && !spec.Nullable {
				return nil, tableArgumentError("missing required column " + column)
			}
		}
	}
	return row, nil
}

func (t *pluginTableRows) get(ctx context.Context, id int64) (map[string]any, error) {
	rows, err := t.query(ctx, entsql.Dialect(t.dialect).Select().Where(entsql.EQ("id", id)))
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (t *pluginTableRows) insert(ctx context.Context, row map[string]any) (map[string]any, error) {
	insert := entsql.Dialect(t.dialect).Insert(t.name)
	for _, column := range slices.Sorted(maps.Keys(row)) {
		insert.Set(column, row[column])
	}
	if len(row) == 0 {
		insert.Default()
	}
	query, args := insert.Returning("id").Query()
	var id int64
	if t.dialect == dialect.MySQL {
		result, err := t.tx.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		id, err = result.LastInsertId()
		if err != nil {
			return nil, err
		}
	} else if err := t.tx.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		return nil, err
	}
	return t.get(ctx, id)
}

func (t *pluginTableRows) update(ctx context.Context, id int64, row map[string]any) (map[string]any, error) {
	if len(row) > 0 {
		update := entsql.Dialect(t.dialect).Update(t.name).Where(entsql.EQ("id", id))
		for _, column := range slices.Sorted(maps.Keys(row)) {
			update.Set(column, row[column])
		}
		query, args := update.Query()
		if _, err := t.tx.ExecContext(ctx, query, args...); err != nil {
			return nil, err
		}
	}
	return t.get(ctx, id)
}

func (t *pluginTableRows) upsert(ctx context.Context, conflict []string, row map[string]any) (map[string]any, error) {
	if !slices.ContainsFunc(t.spec.Indexes, func(index objects.PluginTableIndex) bool {
		return index.Unique && slices.Equal(index.Columns, conflict)
	}) {
		return nil, tableArgumentError("conflict must match a declared unique index")
	}
	builder := entsql.Dialect(t.dialect)
	selectID := builder.Select("id").From(builder.Table(t.name))
	for _, name := range conflict {
		selectID.Where(entsql.EQ(name, row[name]))
	}
	query, args := selectID.Query()
	var id int64
	err := t.tx.QueryRowContext(ctx, query, args...).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return t.insert(ctx, row)
	}
	if err != nil {
		return nil, err
	}
	return t.update(ctx, id, row)
}

func (t *pluginTableRows) query(ctx context.Context, selector *entsql.Selector) ([]map[string]any, error) {
	names := append([]string{"id"}, slices.Sorted(maps.Keys(t.spec.Columns))...)
	query, args := selector.Select(names...).From(entsql.Table(t.name)).Query()
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]any, len(names))
	targets := make([]any, len(names))
	for i := range values {
		targets[i] = &values[i]
	}
	result := make([]map[string]any, 0)
	resultSize := 0
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(names))
		for i, name := range names {
			row[name] = t.resultValue(name, values[i])
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		resultSize += len(encoded) + 1
		if resultSize > pluginSQLBytes {
			return nil, &pluginTableError{hostLimitExceeded, "plugin table result exceeds 1 MiB"}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (t *pluginTableRows) resultValue(name string, value any) any {
	column, _ := t.column(name)
	switch v := value.(type) {
	case int64:
		if column.Type == "boolean" {
			return v != 0
		}
	case []byte:
		if column.Type == "string" || column.Type == "text" {
			return string(v)
		}
	}
	return sqlJSONValue(value, "")
}

func (t *pluginTableRows) list(ctx context.Context, options tableSelectInput) (any, error) {
	if options.First == 0 {
		options.First = 50
	}
	if options.First < 1 || options.First > 100 || len(options.Where) > 8 || len(options.OrderBy) > 4 {
		return nil, tableArgumentError("list accepts 1–100 rows, 8 filters and 4 sort columns")
	}
	order, err := t.listOrder(options.OrderBy)
	if err != nil {
		return nil, err
	}
	predicates, err := t.listFilters(options.Where)
	if err != nil {
		return nil, err
	}
	key, err := json.Marshal(struct {
		Where []tableFilter `json:"Where"`
		Order []tableOrder  `json:"Order"`
	}{options.Where, order})
	if err != nil {
		return nil, err
	}
	hash := sha256Hex(append([]byte(t.name+"|"), key...))
	if options.After != "" {
		predicate, err := t.listCursor(options.After, hash, order)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, predicate)
	}
	selector := entsql.Dialect(t.dialect).Select().Limit(options.First + 1)
	for _, predicate := range predicates {
		selector.Where(predicate)
	}
	for _, field := range order {
		sortColumn := entsql.Asc
		if field.Direction == "desc" {
			sortColumn = entsql.Desc
		}
		selector.OrderBy(sortColumn(field.Column))
	}
	rows, err := t.query(ctx, selector)
	if err != nil {
		return nil, err
	}
	var cursor any
	if len(rows) > options.First {
		rows = rows[:options.First]
		next := tableCursor{Query: hash}
		last := rows[len(rows)-1]
		for _, field := range order {
			raw, err := json.Marshal(last[field.Column])
			if err != nil {
				return nil, err
			}
			next.Values = append(next.Values, raw)
		}
		raw, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		cursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return map[string]any{"rows": rows, "nextCursor": cursor}, nil
}

func (t *pluginTableRows) listOrder(order []tableOrder) ([]tableOrder, error) {
	seen := make(map[string]bool, len(order))
	for _, field := range order {
		column, exists := t.column(field.Column)
		if !exists || column.Nullable || column.Type == "text" || seen[field.Column] ||
			(field.Direction != "asc" && field.Direction != "desc") {
			return nil, tableArgumentError("invalid orderBy column or direction")
		}
		seen[field.Column] = true
	}
	if !seen["id"] {
		order = append(order, tableOrder{Column: "id", Direction: "asc"})
	}
	return order, nil
}

func (t *pluginTableRows) listFilters(filters []tableFilter) ([]*entsql.Predicate, error) {
	operators := map[string]func(string, any) *entsql.Predicate{
		"eq": entsql.EQ, "lt": entsql.LT, "lte": entsql.LTE, "gt": entsql.GT, "gte": entsql.GTE,
	}
	predicates := make([]*entsql.Predicate, 0, len(filters))
	for _, filter := range filters {
		operator := operators[filter.Op]
		if operator == nil {
			return nil, tableArgumentError("invalid filter operator")
		}
		value, err := t.bindValue(filter.Column, filter.Value)
		if err != nil {
			return nil, err
		}
		if value == nil {
			if filter.Op != "eq" {
				return nil, tableArgumentError("null requires the eq filter")
			}
			predicates = append(predicates, entsql.IsNull(filter.Column))
		} else {
			predicates = append(predicates, operator(filter.Column, value))
		}
	}
	return predicates, nil
}

func (t *pluginTableRows) listCursor(input, key string, order []tableOrder) (*entsql.Predicate, error) {
	raw, err := base64.RawURLEncoding.DecodeString(input)
	if err != nil {
		return nil, tableArgumentError("invalid table cursor")
	}
	var cursor tableCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Query != key || len(cursor.Values) != len(order) {
		return nil, tableArgumentError("cursor does not match this query")
	}
	values := make([]any, len(order))
	for i, field := range order {
		value, err := t.bindValue(field.Column, cursor.Values[i])
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	terms := make([]*entsql.Predicate, 0, len(order))
	for i, field := range order {
		parts := make([]*entsql.Predicate, 0, i+1)
		for j := range i {
			parts = append(parts, entsql.EQ(order[j].Column, values[j]))
		}
		compare := entsql.GT
		if field.Direction == "desc" {
			compare = entsql.LT
		}
		parts = append(parts, compare(field.Column, values[i]))
		terms = append(terms, entsql.And(parts...))
	}
	return entsql.Or(terms...), nil
}

func isPluginTableConflict(err error) bool {
	if e, ok := errors.AsType[*mysql.MySQLError](err); ok {
		return e.Number == 1062
	}
	if e, ok := errors.AsType[*pgconn.PgError](err); ok {
		return e.Code == "23505"
	}
	if e, ok := errors.AsType[*sqlite.Error](err); ok {
		return e.Code() == 1555 || e.Code() == 2067
	}
	return false
}
