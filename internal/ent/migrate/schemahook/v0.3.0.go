package schemahook

import (
	"context"

	"entgo.io/ent/dialect/sql/schema"
)

func removeIndex(table *schema.Table, indexName string) *schema.Table {
	var indexes []*schema.Index

	for _, index := range table.Indexes {
		if index.Name == indexName {
			continue
		}

		indexes = append(indexes, index)
	}

	table.Indexes = indexes

	return table
}

func V0_3_0(next schema.Creator) schema.Creator {
	return schema.CreateFunc(func(ctx context.Context, tables ...*schema.Table) error {
		for i, table := range tables {
			if table.Name == "user_projects" {
				table = removeIndex(table, "userproject_project_id_user_id")
				tables[i] = table
			}

			if table.Name == "user_roles" {
				table = removeIndex(table, "userrole_role_id_user_id")
				tables[i] = table
			}
		}

		return next.Create(ctx, tables...)
	})
}
