package backend

import (
	"context"
	"encoding/json"
)

// Actor is descriptive, never sufficient to authenticate a Fabric transaction.
// Fabric identities are always obtained from configured server-side credentials.
type Actor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	MSPID string `json:"mspId"`
	Role  string `json:"role"`
}

type Backend interface {
	Invoke(context.Context, string, string, ...string) (json.RawMessage, error)
	Actors() []Actor
	Mode() string
	Close() error
}

// Catalog is an optional DEMO-only facility; the original contract has no
// state-range/list transaction, so it cannot enumerate Fabric ledger entries.
type Catalog interface {
	List(kind string) ([]json.RawMessage, error)
}
