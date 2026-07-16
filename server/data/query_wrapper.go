// Package data provides the data access layer for Aether.
//
// query_wrapper.go implements a MyBatis-Plus style dynamic query builder
// (reference: ferret project's data/query_wrapper.go, adapted from GORM to pgx).
//
// Design (W1-02 decision D4):
//   - Used ONLY for dynamic queries that sqlc cannot handle well: runtime-determined
//     IN-lists, ad-hoc filter combinations, pagination. Fixed queries MUST go in
//     pkg/db/queries/*.sql (SQL-as-truth).
//   - Backed by Masterminds/squirrel (standard SQL builder, no ORM reflection),
//     producing SQL string + args compatible with pgx's pool.Query(ctx, sql, args...).
//   - Fluent chainable API: New().Eq().In().OrderBy().Page().BuildSQL(base)
package data

import (
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

// Operator describes a comparison in a WHERE clause.
type Operator string

const (
	OpEq         Operator = "="
	OpNe         Operator = "!="
	OpGt         Operator = ">"
	OpGte        Operator = ">="
	OpLt         Operator = "<"
	OpLte        Operator = "<="
	OpLike       Operator = "LIKE"
	OpNotLike    Operator = "NOT LIKE"
	OpIn         Operator = "IN"
	OpNotIn      Operator = "NOT IN"
	OpBetween    Operator = "BETWEEN"
	OpNotBetween Operator = "NOT BETWEEN"
	OpIsNull     Operator = "IS NULL"
	OpIsNotNull  Operator = "IS NOT NULL"
)

// Pagination holds optional page/size. Nil Page field means no pagination.
type Pagination struct {
	Page int // 1-based
	Size int // page size
}

// QueryWrapper builds dynamic WHERE/ORDER/LIMIT clauses on top of a base SQL
// (e.g. "SELECT * FROM modules WHERE deleted_at IS NULL").
//
// Usage:
//
//	w := New().Eq("status", "active").In("layer", "db", "mw").
//	    OrderByDesc("created_at").Page(1, 20)
//	sql, args, err := w.BuildSQL("SELECT * FROM modules WHERE deleted_at IS NULL")
//	rows, err := pool.Query(ctx, sql, args...)
type QueryWrapper struct {
	builder sq.SelectBuilder
	// We keep conditions/orderby/page separate so BuildSQL can compose with a
	// caller-provided base that already contains a WHERE (like deleted_at IS NULL).
	conds    []sq.Sqlizer
	orderBy  []orderClause
	page     *Pagination
	placeholderErr error // captured during construction
}

type orderClause struct {
	column string
	desc   bool
}

// New creates an empty QueryWrapper (AND logic by default).
func New() *QueryWrapper {
	return &QueryWrapper{}
}

// ---- Comparison builders (chainable) ----

// Eq adds `column = value`.
func (w *QueryWrapper) Eq(column string, value any) *QueryWrapper {
	w.conds = append(w.conds, sq.Eq{column: value})
	return w
}

// Ne adds `column != value`.
func (w *QueryWrapper) Ne(column string, value any) *QueryWrapper {
	w.conds = append(w.conds, sq.NotEq{column: value})
	return w
}

// Gt adds `column > value`.
func (w *QueryWrapper) Gt(column string, value any) *QueryWrapper {
	w.conds = append(w.conds, sq.Gt{column: value})
	return w
}

// Gte adds `column >= value`.
func (w *QueryWrapper) Gte(column string, value any) *QueryWrapper {
	w.conds = append(w.conds, sq.GtOrEq{column: value})
	return w
}

// Lt adds `column < value`.
func (w *QueryWrapper) Lt(column string, value any) *QueryWrapper {
	w.conds = append(w.conds, sq.Lt{column: value})
	return w
}

// Le adds `column <= value`.
func (w *QueryWrapper) Le(column string, value any) *QueryWrapper {
	w.conds = append(w.conds, sq.LtOrEq{column: value})
	return w
}

// Like adds `column LIKE value` (caller provides % wildcards).
func (w *QueryWrapper) Like(column string, pattern string) *QueryWrapper {
	w.conds = append(w.conds, sq.Like{column: pattern})
	return w
}

// NotLike adds `column NOT LIKE value`.
func (w *QueryWrapper) NotLike(column string, pattern string) *QueryWrapper {
	w.conds = append(w.conds, sq.NotLike{column: pattern})
	return w
}

// In adds `column IN (values...)`.
func (w *QueryWrapper) In(column string, values ...any) *QueryWrapper {
	w.conds = append(w.conds, sq.Eq{column: values})
	return w
}

// NotIn adds `column NOT IN (values...)`.
func (w *QueryWrapper) NotIn(column string, values ...any) *QueryWrapper {
	w.conds = append(w.conds, sq.NotEq{column: values})
	return w
}

// Between adds `column BETWEEN low AND high`.
func (w *QueryWrapper) Between(column string, low, high any) *QueryWrapper {
	w.conds = append(w.conds, sq.Expr(column+" BETWEEN ? AND ?", low, high))
	return w
}

// IsNull adds `column IS NULL`.
func (w *QueryWrapper) IsNull(column string) *QueryWrapper {
	w.conds = append(w.conds, sq.Eq{column: nil})
	return w
}

// IsNotNull adds `column IS NOT NULL`.
func (w *QueryWrapper) IsNotNull(column string) *QueryWrapper {
	w.conds = append(w.conds, sq.NotEq{column: nil})
	return w
}

// Or wraps the given conditions in a parenthesized OR group:
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
	grouped := make([]sq.Sqlizer, len(sub.conds))
	copy(grouped, sub.conds)
	w.conds = append(w.conds, sq.Or(grouped))
	return w
}

// And wraps the given conditions in a parenthesized AND group. Rarely needed
// (default is AND) but provided for parity with Or.
func (w *QueryWrapper) And(fn func(*QueryWrapper)) *QueryWrapper {
	sub := New()
	fn(sub)
	if len(sub.conds) == 0 {
		return w
	}
	grouped := make([]sq.Sqlizer, len(sub.conds))
	copy(grouped, sub.conds)
	w.conds = append(w.conds, sq.And(grouped))
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
// a WHERE clause is introduced. Uses squirrel's PlaceholderFormat to emit $1/$2/...
// (pgx native format).
func (w *QueryWrapper) BuildSQL(base string) (sql string, args []any, err error) {
	if w.placeholderErr != nil {
		return "", nil, w.placeholderErr
	}

	// Start from a squirrel builder seeded with the base so ORDER BY / LIMIT
	// compose consistently, but we need to preserve the exact base text.
	// Strategy: append WHERE conditions as text into the base, then ORDER BY,
	// then LIMIT/OFFSET, using squirrel only to render conditions + args.
	sqlStr := base

	if len(w.conds) > 0 {
		whereSQL, whereArgs, err := sq.And(w.conds).ToSql()
		if err != nil {
			return "", nil, fmt.Errorf("query_wrapper: build WHERE: %w", err)
		}
		// squirrel returns "WHERE ..."; strip the leading keyword to merge cleanly.
		whereBody := strings.TrimPrefix(whereSQL, "WHERE ")
		if containsWhere(sqlStr) {
			sqlStr += " AND " + whereBody
		} else {
			sqlStr += " WHERE " + whereBody
		}
		args = append(args, whereArgs...)
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
		// LIMIT/OFFSET use the same positional args ($N).
		sqlStr += fmt.Sprintf(" LIMIT %d OFFSET %d", w.page.Size, offset)
	}

	// Convert ? placeholders to $N (pgx). squirrel's PlaceholderFormat would do
	// this if we built the whole thing via the builder, but since we splice text
	// manually we normalize here.
	sqlStr, args, err = normalizePlaceholders(sqlStr, args)
	if err != nil {
		return "", nil, err
	}
	return sqlStr, args, nil
}

// containsWhere reports whether the base SQL already has a WHERE clause
// (case-insensitive, naive word-boundary match — sufficient for generated bases).
func containsWhere(s string) bool {
	upper := strings.ToUpper(s)
	idx := strings.Index(upper, "WHERE")
	if idx < 0 {
		return false
	}
	// Ensure word boundary (preceding space or start).
	if idx > 0 {
		prev := s[idx-1]
		if prev != ' ' && prev != '\t' && prev != '\n' {
			return false
		}
	}
	return true
}

// normalizePlaceholders converts "? $1"-style mixed placeholders to pure $N
// positional placeholders expected by pgx, reordering args accordingly.
// We rebuild the placeholder list based on the count of args.
func normalizePlaceholders(sqlStr string, args []any) (string, []any, error) {
	// Count ? in the SQL and ensure they match args length for the WHERE part.
	// LIMIT/OFFSET are already inlined as literals, so only ? from conditions remain.
	var out strings.Builder
	argIdx := 0
	i := 0
	for i < len(sqlStr) {
		if sqlStr[i] == '?' {
			argIdx++
			if argIdx > len(args) {
				return "", nil, fmt.Errorf("query_wrapper: more placeholders (%d) than args (%d)", argIdx, len(args))
			}
			out.WriteString("$")
			out.WriteString(fmt.Sprintf("%d", argIdx))
			i++
			continue
		}
		out.WriteByte(sqlStr[i])
		i++
	}
	// args stay in the order conditions were appended (squirrel returns them in order).
	return out.String(), args, nil
}
