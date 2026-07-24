// Package dbset provides multi-table aggregate structs (W1-02 decision D5).
//
// dbset structs compose multiple sqlc-generated models into a single view struct
// for core consumers that need cross-table data (e.g. stack list with space name +
// layer name). They are opt-in: only build a dbset when a core consumer needs the
// aggregate; single-table operations use Repo directly.
//
// When to add a dbset (decision rule):
//   - A core consumer (e.g. stackmodel list view) needs a JOIN-ed result that
//     spans 2+ tables AND the result shape is reused in multiple call sites.
//   - The population method lives in the relevant Repo (e.g. StackRepo.ListWithSpace),
//     implemented via either a sqlc query with sqlc.embed() for nested shapes OR
//     a query_wrapper-based JOIN. NOT by re-querying each table separately.
//
// W1-02 deliberately ships NO dbset structs (YAGNI): the first consumer is W1-04
// stackmodel. When W1-04 lands, add StackWithSpace here + ListWithSpace on
// StackRepo with a JOIN query in pkg/db/queries/stacks.sql.
package dbset
