package db

import (
	"strings"

	aschema "ariga.io/atlas/sql/schema"
	eschema "entgo.io/ent/dialect/sql/schema"
)

// filterEquivalentDefaultChanges returns a diff hook that filters out ChangeDefault
// changes where the current and desired defaults only differ by formatting
// (outer parentheses, or MySQL's now() spelling of CURRENT_TIMESTAMP).
//
// Without this, ent wraps Expr defaults (e.g. entsql.DefaultExpr("CURRENT_TIMESTAMP"))
// in parentheses when building the desired schema, while SQLite/MySQL report the
// stored default without them (CURRENT_TIMESTAMP / now()), so a ChangeDefault is
// computed on every startup. SQLite then rebuilds the whole table to "fix" the
// default (it cannot ALTER one), which on a partially cleaned table resets its
// AUTOINCREMENT sequence to MAX(id) and causes request ids to be reused. MySQL
// merely runs a redundant ALTER each startup. PostgreSQL is unaffected: its
// driver compares defaults semantically. Real default changes still pass through.
func filterEquivalentDefaultChanges() eschema.MigrateOption {
	return eschema.WithDiffHook(func(next eschema.Differ) eschema.Differ {
		return eschema.DiffFunc(func(current, desired *aschema.Schema) ([]aschema.Change, error) {
			changes, err := next.Diff(current, desired)
			if err != nil {
				return nil, err
			}
			return filterEquivalentDefaultChangesFrom(changes), nil
		})
	})
}

// filterEquivalentDefaultChangesFrom prunes equivalent default-only changes
// from the computed diff and drops tables whose changes become empty.
func filterEquivalentDefaultChangesFrom(changes []aschema.Change) []aschema.Change {
	filtered := make([]aschema.Change, 0, len(changes))
	for _, c := range changes {
		mt, ok := c.(*aschema.ModifyTable)
		if !ok {
			filtered = append(filtered, c)
			continue
		}
		kept := make([]aschema.Change, 0, len(mt.Changes))
		for _, ic := range mt.Changes {
			mc, ok := ic.(*aschema.ModifyColumn)
			if !ok || !isEquivalentDefaultChange(mc) {
				kept = append(kept, ic)
				continue
			}
		}
		if len(kept) == 0 {
			// The whole table-level change was a no-op after filtering.
			continue
		}
		mt.Changes = kept
		filtered = append(filtered, mt)
	}
	return filtered
}

// isEquivalentDefaultChange reports whether the only change on mc is the
// default value and the current/desired defaults are semantically equivalent.
func isEquivalentDefaultChange(mc *aschema.ModifyColumn) bool {
	// Only touch pure default changes; any other column change
	// (type, nullability, generated, charset, ...) must be left untouched.
	if mc.Change != aschema.ChangeDefault {
		return false
	}
	from, ok := columnDefault(mc.From)
	if !ok {
		return false
	}
	to, ok := columnDefault(mc.To)
	if !ok {
		return false
	}
	return normalizeDefault(from) == normalizeDefault(to)
}

// columnDefault returns the string form of the column default value.
func columnDefault(c *aschema.Column) (string, bool) {
	if c == nil || c.Default == nil {
		return "", false
	}
	switch x := aschema.UnderlyingExpr(c.Default).(type) {
	case *aschema.Literal:
		return x.V, true
	case *aschema.RawExpr:
		return x.X, true
	default:
		return "", false
	}
}

// normalizeDefault returns a canonical form used to compare default values:
// strips outer parentheses and unifies the timestamp spellings
// (CURRENT_TIMESTAMP/CURRENT_TIMESTAMP()/now()) into one form. Quoted
// literals are left untouched.
func normalizeDefault(s string) string {
	s = strings.TrimSpace(s)
	// Strip one level of semantically insignificant outer parentheses,
	// e.g. "(CURRENT_TIMESTAMP)" -> "CURRENT_TIMESTAMP".
	for strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && fullyParenWrapped(s) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	// MySQL reports DEFAULT (CURRENT_TIMESTAMP) columns as now().
	switch strings.ToLower(s) {
	case "current_timestamp", "current_timestamp()", "now()":
		return "current_timestamp"
	default:
		return s
	}
}

// fullyParenWrapped reports whether s is fully enclosed by a balanced pair of
// outermost parentheses. Parentheses inside single-quoted literals are ignored.
func fullyParenWrapped(s string) bool {
	depth := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			// A doubled quote is an escaped quote inside the literal.
			if inQuote && i+1 < len(s) && s[i+1] == '\'' {
				i++
				continue
			}
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i < len(s)-1 {
				return false
			}
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}
