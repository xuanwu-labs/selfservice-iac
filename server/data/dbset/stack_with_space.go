// Package dbset provides multi-table aggregate structs (W1-02 decision D5).
//
// dbset structs compose multiple sqlc-generated models into a single view struct
// for core consumers that need cross-table data (e.g. stack list with space name +
// layer name). They are opt-in: only build a dbset when a core consumer needs the
// aggregate; single-table operations use Repo directly.
//
// Data acquisition is via Repo methods (sqlc.embed() JOIN or query_wrapper),
// NOT by re-querying each table separately.
package dbset

import (
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// StackWithSpace aggregates a stack with its optional space and layer info.
// Used by W1-04 stackmodel list view (show stack + space name + layer display name).
//
// Space may be nil (stacks.space_id is NULL when a stack has no space).
// Layer may be nil (stacks.layer_logical_id is NULL theoretically, though in practice
// always set for applied stacks).
type StackWithSpace struct {
	Stack generated.Stack        `json:"stack"`
	Space *generated.Space       `json:"space,omitempty"`
	Layer *generated.LayerLogicalRef `json:"layer,omitempty"`
}
