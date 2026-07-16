// Package repo implements data access Repo structs (W1-02 hybrid paradigm:
// ferret Repo struct × DIP evolvable × sqlc SQL-as-truth). Each Repo is a thin
// wrapper over *generated.Queries, registered in data.ProviderSet.
//
// Design (W1-02 decisions D1-D4):
//   - D1: Repo struct holds pool + queries. Plain struct (ferret style); core
//     injects it directly via wire. If a core consumer needs to mock, extract a
//     small interface there (Go implicit interfaces).
//   - D2: Fixed queries live in pkg/db/queries/*.sql and are compiled by sqlc
//     into *generated.Queries methods (SQL-as-truth). Repo methods just forward.
//   - D3: Cross-table mutations use r.pool.Begin + r.queries.WithTx + Commit,
//     with deferred Rollback (no-op after Commit).
//   - D4: Dynamic queries (IN-lists, ad-hoc multi-filter, pagination) that
//     sqlc cannot express go through ListByDynamicFilter, which builds SQL with
//     *data.QueryWrapper and runs it via r.pool.Query + pgx.CollectRows.
package repo
