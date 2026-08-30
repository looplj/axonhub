package db

import (
	"testing"

	aschema "ariga.io/atlas/sql/schema"
)

func TestNormaliizeDefault(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"CURRENT_TIMESTAMP", "current_timestamp"},
		{"(CURRENT_TIMESTAMP)", "current_timestamp"},
		{"now()", "current_timestamp"},
		{"(now())", "current_timestamp"},
		{"NOW()", "current_timestamp"},
		{"CURRENT_TIMESTAMP()", "current_timestamp"},
		{"('2026-01-01')", "'2026-01-01'"},
		{"(1 + 2)", "1 + 2"},
		{"0", "0"},
		{"(('nested'))", "'nested'"},
		// Quoted literals must never be normalized into keywords.
		{"'now()'", "'now()'"},
		{"'CURRENT_TIMESTAMP'", "'CURRENT_TIMESTAMP'"},
		{"'Pending'", "'Pending'"},
		{"'pending'", "'pending'"},
		// Parentheses inside quoted literals are ignored when unwrapping.
		{"'('", "'('"},
		{"('(')", "'('"},
		{"(')')", "')'"},
		{"'(('", "'(('"},
		{"(('a''b'))", "'a''b'"},
	}
	for _, tt := range tests {
		if got := normalizeDefault(tt.in); got != tt.want {
			t.Errorf("normalizeDefault(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsEquivalentDefaultChange(t *testing.T) {
	col := func(name, def string) *aschema.Column {
		c := &aschema.Column{Name: name}
		if def != "" {
			c.Default = &aschema.RawExpr{X: def}
		}
		return c
	}

	tests := []struct {
		name string
		from *aschema.Column
		to   *aschema.Column
		want bool
	}{
		{
			name: "paren difference IS equivalent",
			from: col("created_at", "CURRENT_TIMESTAMP"),
			to:   col("created_at", "(CURRENT_TIMESTAMP)"),
			want: true,
		},
		{
			name: "mysql now() IS equivalent to paren form",
			from: col("created_at", "now()"),
			to:   col("created_at", "(CURRENT_TIMESTAMP)"),
			want: true,
		},
		{
			name: "identical",
			from: col("created_at", "CURRENT_TIMESTAMP"),
			to:   col("created_at", "CURRENT_TIMESTAMP"),
			want: true,
		},
		{
			name: "real default change NOT equivalent",
			from: col("status", "active"),
			to:   col("status", "disabled"),
			want: false,
		},
		{
			name: "not both present NOT equivalent",
			from: col("created_at", "CURRENT_TIMESTAMP"),
			to:   col("created_at", ""),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &aschema.ModifyColumn{From: tt.from, To: tt.to, Change: aschema.ChangeDefault}
			if got := isEquivalentDefaultChange(mc); got != tt.want {
				t.Errorf("isEquivalentDefaultChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterEquivalentDefaultChangesFrom(t *testing.T) {
	col := func(name, def string) *aschema.Column {
		c := &aschema.Column{Name: name}
		if def != "" {
			c.Default = &aschema.RawExpr{X: def}
		}
		return c
	}
	modifyTable := func(name string, changes ...aschema.Change) *aschema.ModifyTable {
		mt := &aschema.ModifyTable{T: &aschema.Table{Name: name}}
		mt.Changes = changes
		return mt
	}

	changes := []aschema.Change{
		&aschema.AddTable{T: &aschema.Table{Name: "brand_new"}},
		modifyTable("only_default_paren_diff",
			&aschema.ModifyColumn{From: col("created_at", "CURRENT_TIMESTAMP"), To: col("created_at", "(CURRENT_TIMESTAMP)"), Change: aschema.ChangeDefault},
		),
		modifyTable("real_change",
			&aschema.ModifyColumn{From: col("status", "active"), To: col("status", "disabled"), Change: aschema.ChangeDefault},
			&aschema.ModifyColumn{From: col("created_at", "CURRENT_TIMESTAMP"), To: col("created_at", "(CURRENT_TIMESTAMP)"), Change: aschema.ChangeDefault},
		),
	}

	out := filterEquivalentDefaultChangesFrom(changes)
	if len(out) != 2 {
		t.Fatalf("got %d changes, want 2 (AddTable + real_change)", len(out))
	}
	// AddTable must pass through untouched.
	if _, ok := out[0].(*aschema.AddTable); !ok {
		t.Errorf("first change should be AddTable, got %T", out[0])
	}
	mt, ok := out[1].(*aschema.ModifyTable)
	if !ok {
		t.Fatalf("second change should be ModifyTable, got %T", out[1])
	}
	if mt.T.Name != "real_change" {
		t.Errorf("got table %q, want real_change", mt.T.Name)
	}
	if len(mt.Changes) != 1 {
		t.Errorf("real_change should keep exactly 1 inner change, got %d", len(mt.Changes))
	}
}
