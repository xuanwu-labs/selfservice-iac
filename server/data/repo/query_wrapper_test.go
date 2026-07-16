package repo_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/data/repo"
)

// TestQueryWrapper_BasicConditions verifies Eq/In/Like produce correct SQL + args.
func TestQueryWrapper_BasicConditions(t *testing.T) {
	w := repo.New().
		Eq("status", "active").
		In("layer", "db", "mw").
		Like("name", "%order%")

	sql, args, err := w.BuildSQL("SELECT * FROM modules")
	require.NoError(t, err)

	// base has no WHERE, so wrapper introduces one
	require.True(t, strings.Contains(strings.ToUpper(sql), "WHERE"),
		"sql should contain WHERE: %s", sql)

	// args order: status, layer(in:2 values), name(like)
	require.Len(t, args, 4)
	assert.Equal(t, "active", args[0])
	assert.Equal(t, "db", args[1])
	assert.Equal(t, "mw", args[2])
	assert.Equal(t, "%order%", args[3])

	// placeholders are $N format (pgx)
	assert.Contains(t, sql, "$1")
	assert.Contains(t, sql, "$2")
}

// TestQueryWrapper_AppendsToExistingWhere verifies that when base already has
// WHERE (e.g. "deleted_at IS NULL"), wrapper appends with AND.
func TestQueryWrapper_AppendsToExistingWhere(t *testing.T) {
	w := repo.New().Eq("status", "active").OrderByDesc("created_at").Page(1, 20)

	sql, args, err := w.BuildSQL("SELECT * FROM modules WHERE deleted_at IS NULL")
	require.NoError(t, err)

	// should append " AND status = $1" not introduce a second WHERE
	assert.Contains(t, sql, "deleted_at IS NULL AND")
	assert.NotContains(t, strings.ToUpper(sql), "WHERE WHERE")

	// ORDER BY + LIMIT/OFFSET present
	assert.Contains(t, sql, "ORDER BY created_at DESC")
	assert.Contains(t, sql, "LIMIT 20 OFFSET 0")

	// only 1 arg (status); LIMIT/OFFSET are inlined as literals
	require.Len(t, args, 1)
	assert.Equal(t, "active", args[0])
}

// TestQueryWrapper_NestedOr verifies Or(fn) creates a parenthesized OR group.
func TestQueryWrapper_NestedOr(t *testing.T) {
	w := repo.New().
		Eq("a", 1).
		Or(func(sub *repo.QueryWrapper) {
			sub.Eq("b", 2).Eq("c", 3)
		})

	sql, args, err := w.BuildSQL("SELECT * FROM t")
	require.NoError(t, err)

	// a = $1 OR (b = $2 AND c = $3)
	assert.Contains(t, sql, "OR (")
	assert.Contains(t, sql, "AND")
	require.Len(t, args, 3)
	assert.Equal(t, 1, args[0])
	assert.Equal(t, 2, args[1])
	assert.Equal(t, 3, args[2])
}

// TestQueryWrapper_Pagination verifies page/size math.
func TestQueryWrapper_Pagination(t *testing.T) {
	tests := []struct {
		name        string
		page, size  int
		wantOffset  int
		wantLiteral string
	}{
		{"page1_size10", 1, 10, 0, "LIMIT 10 OFFSET 0"},
		{"page2_size20", 2, 20, 20, "LIMIT 20 OFFSET 20"},
		{"page3_size15", 3, 15, 30, "LIMIT 15 OFFSET 30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := repo.New().Page(tt.page, tt.size)
			sql, _, err := w.BuildSQL("SELECT * FROM t")
			require.NoError(t, err)
			assert.Contains(t, sql, tt.wantLiteral)
		})
	}
}

// TestQueryWrapper_DisablePage verifies pagination can be cleared.
func TestQueryWrapper_DisablePage(t *testing.T) {
	w := repo.New().Page(1, 10).DisablePage()
	sql, _, err := w.BuildSQL("SELECT * FROM t")
	require.NoError(t, err)
	assert.NotContains(t, strings.ToUpper(sql), "LIMIT")
}

// TestQueryWrapper_Empty verifies empty wrapper returns base unchanged.
func TestQueryWrapper_Empty(t *testing.T) {
	w := repo.New()
	sql, args, err := w.BuildSQL("SELECT * FROM modules")
	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM modules", sql)
	assert.Empty(t, args)
}

// TestQueryWrapper_Between verifies BETWEEN renders correctly.
func TestQueryWrapper_Between(t *testing.T) {
	w := repo.New().Between("age", 18, 60)
	sql, args, err := w.BuildSQL("SELECT * FROM users")
	require.NoError(t, err)
	assert.Contains(t, sql, "BETWEEN")
	require.Len(t, args, 2)
	assert.Equal(t, 18, args[0])
	assert.Equal(t, 60, args[1])
}

// TestQueryWrapper_IsNull verifies IS NULL / IS NOT NULL.
func TestQueryWrapper_IsNull(t *testing.T) {
	w := repo.New().IsNull("deleted_at").IsNotNull("email")
	sql, args, err := w.BuildSQL("SELECT * FROM users")
	require.NoError(t, err)
	assert.Contains(t, sql, "IS NULL")
	assert.Contains(t, sql, "IS NOT NULL")
	// IS NULL/IS NOT NULL produce args (squirrel represents them with nil value)
	// Just verify the SQL renders without error.
	_ = args
}
