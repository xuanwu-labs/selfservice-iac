// Package data provides the data access layer for Aether.
//
// query_wrapper.go implements a MyBatis-Plus style dynamic query builder
// (reference: ferret project's data/query_wrapper.go, adapted from GORM to pgx).
//
// Design (W1-02 decision D4):
//   - Used ONLY for dynamic queries that sqlc cannot handle well: runtime-determined
//     IN-lists, ad-hoc filter combinations, pagination. Fixed queries MUST go in
//     pkg/db/queries/*.sql (SQL-as-truth).
//   - Uses Masterminds/squirrel only for rendering individual condition clauses
//     (placeholder + args). The AND/OR join logic is hand-rolled to match the
//     MyBatis-Plus semantics where Or(fn) makes the GROUP join with OR (not the
//     conditions inside the group).
//   - Produces SQL string + args compatible with pgx's pool.Query(ctx, sql, args...).
package repo

import (
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

// Pagination holds optional page/size. Nil Page field means no pagination.
type Pagination struct {
	Page int // 1-based
	Size int // page size
}

// joinLogic determines how a condition/group connects to the PREVIOUS one.
type joinLogic int

const (
	logicAnd joinLogic = iota
	logicOr
)

// condEntry is a flat condition or a nested group, with the join logic that
// connects it to the previous entry (AND or OR). This flat structure lets us
// implement MyBatis-Plus semantics exactly: Or(fn) sets the group's joinLogic
// to OR (group internals stay AND).
type condEntry struct {
	join joinLogic
	// one of sqlizer / group is set
	sqlizer sq.Sqlizer
	group   []condEntry // nested group (joined internally by AND by default)
}

// QueryWrapper builds dynamic WHERE/ORDER/LIMIT clauses on top of a base SQL
// (e.g. "SELECT * FROM modules WHERE deleted_at IS NULL").
//
// Usage:
//
//	w := New().Eq("status", "active").In("layer", "db", "mw").
//	    OrderByDesc("created_at").Page(1, 20)
//	sql, args, err := w.BuildSQL("SELECT * FROM modules WHERE deleted_at IS NULL")
type QueryWrapper struct {
	conds      []condEntry
	nextJoin   joinLogic // join for the NEXT condition (Or() flips to OR)
	orderBy    []orderClause
	page       *Pagination
	groupInner joinLogic // join inside Or(fn)/And(fn) groups; default AND
}

type orderClause struct {
	column string
	desc   bool
}

// New creates an empty QueryWrapper (AND logic by default).
func New() *QueryWrapper {
	return &QueryWrapper{groupInner: logicAnd}
}

// addCond appends a condition using nextJoin, then resets nextJoin to AND.
func (w *QueryWrapper) addCond(s sq.Sqlizer) *QueryWrapper {
	j := w.nextJoin
	// First condition has no predecessor; join is effectively AND (no prefix).
	w.conds = append(w.conds, condEntry{join: j, sqlizer: s})
	w.nextJoin = logicAnd
	return w
}

// ---- Comparison builders (chainable) ----

// Eq adds `column = value`.
func (w *QueryWrapper) Eq(column string, value any) *QueryWrapper {
	return w.addCond(sq.Eq{column: value})
}

// Ne adds `column != value`.
func (w *QueryWrapper) Ne(column string, value any) *QueryWrapper {
	return w.addCond(sq.NotEq{column: value})
}

// Gt adds `column > value`.
func (w *QueryWrapper) Gt(column string, value any) *QueryWrapper {
	return w.addCond(sq.Gt{column: value})
}

// Gte adds `column >= value`.
func (w *QueryWrapper) Gte(column string, value any) *QueryWrapper {
	return w.addCond(sq.GtOrEq{column: value})
}

// Lt adds `column < value`.
func (w *QueryWrapper) Lt(column string, value any) *QueryWrapper {
	return w.addCond(sq.Lt{column: value})
}

// Le adds `column <= value`.
func (w *QueryWrapper) Le(column string, value any) *QueryWrapper {
	return w.addCond(sq.LtOrEq{column: value})
}

// Like adds `column LIKE value` (caller provides % wildcards).
func (w *QueryWrapper) Like(column string, pattern string) *QueryWrapper {
	return w.addCond(sq.Like{column: pattern})
}

// NotLike adds `column NOT LIKE value`.
func (w *QueryWrapper) NotLike(column string, pattern string) *QueryWrapper {
	return w.addCond(sq.NotLike{column: pattern})
}

// In adds `column IN (values...)`.
func (w *QueryWrapper) In(column string, values ...any) *QueryWrapper {
	return w.addCond(sq.Eq{column: values})
}

// NotIn adds `column NOT IN (values...)`.
func (w *QueryWrapper) NotIn(column string, values ...any) *QueryWrapper {
	return w.addCond(sq.NotEq{column: values})
}

// Between adds `column BETWEEN low AND high`.
func (w *QueryWrapper) Between(column string, low, high any) *QueryWrapper {
	return w.addCond(sq.Expr(column+" BETWEEN ? AND ?", low, high))
}

// IsNull adds `column IS NULL`.
func (w *QueryWrapper) IsNull(column string) *QueryWrapper {
	return w.addCond(sq.Eq{column: nil})
}

// IsNotNull adds `column IS NOT NULL`.
func (w *QueryWrapper) IsNotNull(column string) *QueryWrapper {
	return w.addCond(sq.NotEq{column: nil})
}

// Or wraps the given conditions in a parenthesized group that joins with the
// PREVIOUS condition via OR (MyBatis-Plus semantics). Group internals default
// to AND.
//
//	w.Eq("a", 1).Or(func(sub *QueryWrapper) {
//	    sub.Eq("b", 2).Eq("c", 3)
//	})
//	=> a = $1 OR (b = $2 AND c = $3)
func (w *QueryWrapper) Or(fn func(*QueryWrapper)) *QueryWrapper {
	sub := New()
	fn(sub)
	if len(sub.conds) == 0 {
		return w
	}
	// Group joins with the previous condition via OR.
	w.conds = append(w.conds, condEntry{join: logicOr, group: sub.conds})
	w.nextJoin = logicAnd
	return w
}

// And wraps the given conditions in a parenthesized group that joins with the
// PREVIOUS condition via AND. Rarely needed (default is AND) but provided for
// parity with Or.
func (w *QueryWrapper) And(fn func(*QueryWrapper)) *QueryWrapper {
	sub := New()
	fn(sub)
	if len(sub.conds) == 0 {
		return w
	}
	w.conds = append(w.conds, condEntry{join: logicAnd, group: sub.conds})
	w.nextJoin = logicAnd
	return w
}

// ---- Ordering & pagination ----

// OrderBy appends `column ASC|DESC`.
func (w *QueryWrapper) OrderBy(column string, desc bool) *QueryWrapper {
	w.orderBy = append(w.orderBy, orderClause{column: column, desc: desc})
	return w
}

// OrderByAsc is shorthand for OrderBy(column, false).
func (w *QueryWrapper) OrderByAsc(column string) *QueryWrapper {
	return w.OrderBy(column, false)
}

// OrderByDesc is shorthand for OrderBy(column, true).
func (w *QueryWrapper) OrderByDesc(column string) *QueryWrapper {
	return w.OrderBy(column, true)
}

// Page enables pagination (1-based page index).
func (w *QueryWrapper) Page(page, size int) *QueryWrapper {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	w.page = &Pagination{Page: page, Size: size}
	return w
}

// DisablePage clears any pagination set via Page().
func (w *QueryWrapper) DisablePage() *QueryWrapper {
	w.page = nil
	return w
}

// ---- Build ----

// BuildSQL composes the final SQL and args by appending the wrapper's WHERE,
// ORDER BY and LIMIT/OFFSET clauses to the caller-provided base SQL.
//
// base is the starting query (e.g. "SELECT * FROM modules WHERE deleted_at IS NULL").
// If base already contains a WHERE, new conditions are appended with AND; otherwise
// a WHERE clause is introduced. Uses $1/$2/... placeholders (pgx native format).
func (w *QueryWrapper) BuildSQL(base string) (sql string, args []any, err error) {
	sqlStr := base
	var allArgs []any

	if len(w.conds) > 0 {
		whereBody, err := renderConds(w.conds, &allArgs)
		if err != nil {
			return "", nil, fmt.Errorf("query_wrapper: render WHERE: %w", err)
		}
		if containsWhere(sqlStr) {
			sqlStr += " AND " + whereBody
		} else {
			sqlStr += " WHERE " + whereBody
		}
	}

	if len(w.orderBy) > 0 {
		parts := make([]string, 0, len(w.orderBy))
		for _, o := range w.orderBy {
			dir := "ASC"
			if o.desc {
				dir = "DESC"
			}
			parts = append(parts, o.column+" "+dir)
		}
		sqlStr += " ORDER BY " + strings.Join(parts, ", ")
	}

	if w.page != nil {
		offset := (w.page.Page - 1) * w.page.Size
		sqlStr += fmt.Sprintf(" LIMIT %d OFFSET %d", w.page.Size, offset)
	}

	// Convert ? placeholders to $N (pgx native).
	sqlStr, err = normalizePlaceholders(sqlStr, allArgs)
	if err != nil {
		return "", nil, err
	}
	return sqlStr, allArgs, nil
}

// renderConds builds the WHERE body (without the "WHERE" keyword) from a list
// of condEntry. The first entry has no join prefix; subsequent entries are
// prefixed with AND/OR per their join field. Nested groups are parenthesized.
// Placeholders are ? (squirrel default); normalized to $N by BuildSQL.
// Args are appended in order to *args.
func renderConds(entries []condEntry, args *[]any) (string, error) {
	var parts []string
	for i, e := range entries {
		var part string
		if e.group != nil {
			// Recurse into nested group, parenthesized.
			inner, err := renderConds(e.group, args)
			if err != nil {
				return "", err
			}
			part = "(" + inner + ")"
		} else {
			sql, sqlArgs, err := e.sqlizer.ToSql()
			if err != nil {
				return "", err
			}
			// squirrel returns "column OP ?" — collect args in order.
			*args = append(*args, sqlArgs...)
			part = sql
		}
		if i == 0 {
			// First condition: no join prefix.
			parts = append(parts, part)
		} else {
			op := "AND"
			if e.join == logicOr {
				op = "OR"
			}
			parts = append(parts, op+" "+part)
		}
	}
	return strings.Join(parts, " "), nil
}

// containsWhere reports whether the base SQL already has a WHERE clause
// (case-insensitive, naive word-boundary match — sufficient for generated bases).
func containsWhere(s string) bool {
	upper := strings.ToUpper(s)
	idx := strings.Index(upper, "WHERE")
	if idx < 0 {
		return false
	}
	if idx > 0 {
		prev := s[idx-1]
		if prev != ' ' && prev != '\t' && prev != '\n' {
			return false
		}
	}
	return true
}

// normalizePlaceholders converts ? placeholders to $N positional (pgx format).
func normalizePlaceholders(sqlStr string, args []any) (string, error) {
	var out strings.Builder
	argIdx := 0
	for i := 0; i < len(sqlStr); i++ {
		if sqlStr[i] == '?' {
			argIdx++
			if argIdx > len(args) {
				return "", fmt.Errorf("query_wrapper: more placeholders (%d) than args (%d)", argIdx, len(args))
			}
			out.WriteString("$")
			out.WriteString(fmt.Sprintf("%d", argIdx))
			continue
		}
		out.WriteByte(sqlStr[i])
	}
	return out.String(), nil
}
